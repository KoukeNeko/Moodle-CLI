<?php
// Export the external-function contract installed in one Moodle container.
//
// This runs inside the disposable Docker site. It deliberately asks Moodle's
// own external_api implementation for the descriptions instead of parsing
// db/services.php: inherited descriptions, deprecations and runtime plugin
// registration are otherwise easy to miss.

define('CLI_SCRIPT', true);
require '/var/www/html/config.php';

use core_external\external_description;
use core_external\external_multiple_structure;
use core_external\external_single_structure;
use core_external\external_value;

/** Return a deterministic list from Moodle's comma-delimited metadata. */
function registry_list(?string $raw): array {
    if ($raw === null || trim($raw) === '') {
        return [];
    }
    $items = array_values(array_filter(array_map('trim', explode(',', $raw))));
    sort($items, SORT_STRING);
    return $items;
}

/** Map Moodle PARAM_* names to JSON Schema primitives. */
function registry_value_schema(external_value $description): array {
    $type = strtolower((string)$description->type);
    if ($type === 'bool' || $type === 'boolean') {
        $schema = ['type' => 'boolean'];
    } else if ($type === 'int' || $type === 'integer') {
        $schema = ['type' => 'integer'];
    } else if ($type === 'float' || $type === 'number') {
        $schema = ['type' => 'number'];
    } else if ($type === 'raw') {
        // PARAM_RAW is intentionally not a string contract. Plugins use it
        // for scalar values of several kinds and Moodle performs validation.
        $schema = ['type' => ['string', 'number', 'integer', 'boolean']];
    } else {
        $schema = ['type' => 'string'];
    }
    $schema['x-moodle-param-type'] = (string)$description->type;
    return $schema;
}

/** Make nullability part of JSON Schema, not only Moodle extension metadata. */
function registry_allow_null(array $schema): array {
    if (isset($schema['type']) && is_string($schema['type'])) {
        $schema['type'] = [$schema['type'], 'null'];
    } else if (isset($schema['type']) && is_array($schema['type'])) {
        if (!in_array('null', $schema['type'], true)) {
            $schema['type'][] = 'null';
        }
    } else {
        $schema = ['anyOf' => [$schema, ['type' => 'null']]];
    }
    return $schema;
}

/** Convert Moodle's recursive external_description tree into JSON Schema. */
function registry_schema(?external_description $description, bool $root = false): array {
    if ($description === null) {
        return ['type' => 'null'];
    }
    if ($description instanceof external_value) {
        $schema = registry_value_schema($description);
    } else if ($description instanceof external_multiple_structure) {
        $schema = [
            'type' => 'array',
            'items' => registry_schema($description->content),
        ];
    } else if ($description instanceof external_single_structure) {
        $properties = [];
        $required = [];
        foreach ($description->keys as $name => $child) {
            $properties[$name] = registry_schema($child);
            if ($child->required === VALUE_REQUIRED) {
                $required[] = $name;
            }
        }
        ksort($properties, SORT_STRING);
        sort($required, SORT_STRING);
        $schema = [
            'type' => 'object',
            'properties' => $properties === [] ? (object)[] : $properties,
            'additionalProperties' => false,
        ];
        if ($required !== []) {
            $schema['required'] = $required;
        }
    } else {
        throw new RuntimeException('Unknown external description: ' . get_class($description));
    }

    if ($root) {
        $schema['$schema'] = 'https://json-schema.org/draft/2020-12/schema';
    }
    if ((string)$description->desc !== '') {
        $schema['description'] = trim((string)$description->desc);
    }
    if ($description->allownull) {
        $schema = registry_allow_null($schema);
        $schema['x-moodle-allow-null'] = true;
    }
    if ($description->required === VALUE_DEFAULT) {
        $schema['default'] = $description->default;
    }
    $schema['x-moodle-required'] = (int)$description->required;
    return $schema;
}

/** Credentials are effects even when services.php calls the function read. */
function registry_credential_effect(string $name): bool {
    return str_contains($name, '_get_token')
        || str_contains($name, '_create_token')
        || str_contains($name, '_export_token')
        || str_contains($name, '_autologin_key')
        || str_contains($name, '_private_key');
}

/** Conservative destructive classification used by generic clients and MCP. */
function registry_destructive_effect(string $name, string $effect): bool {
    if ($effect !== 'write') {
        return false;
    }
    return preg_match('/(^|_)(delete|remove|unenrol|unassign|revoke|purge|reset|cancel)(_|$)/', $name) === 1;
}

/** Name the local stub family needed by functions that call another system. */
function registry_external_dependency(string $name, string $component): string {
    if (str_starts_with($component, 'ai') || str_contains($name, '_ai_')) {
        return 'ai';
    }
    if (str_starts_with($component, 'paygw_') || str_contains($name, '_payment_')) {
        return 'payment';
    }
    if (str_starts_with($component, 'sms') || str_contains($name, '_sms_')) {
        return 'sms';
    }
    if (str_starts_with($component, 'ltiservice_') || str_starts_with($component, 'mod_lti')) {
        return 'lti';
    }
    return 'none';
}

global $CFG, $DB;
$records = $DB->get_records('external_functions', null, 'name ASC');
$functions = [];
foreach ($records as $record) {
    try {
        $info = \core_external\external_api::external_function_info($record);
        $credential = registry_credential_effect($info->name);
        $declaredtype = strtolower((string)($info->type ?? ''));
        $effect = $declaredtype === 'read' && !$credential ? 'read' : 'write';
        $component = (string)$info->component;
        $functions[] = [
            'name' => (string)$info->name,
            'component' => $component,
            'description' => $info->description === null ? null : trim((string)$info->description),
            'deprecated' => !empty($info->deprecated),
            'effect' => $effect,
            'destructive' => registry_destructive_effect((string)$info->name, $effect),
            'credential' => $credential,
            'retry' => $effect === 'read' ? 'safe' : 'never',
            'capabilities' => registry_list($info->capabilities ?? ''),
            'services' => registry_list($info->services ?? ''),
            'transports' => [
                'rest' => true,
                'ajax' => !empty($info->allowed_from_ajax),
            ],
            'login_required' => !isset($info->loginrequired) || (bool)$info->loginrequired,
            'external_dependency' => registry_external_dependency((string)$info->name, $component),
            'parameters' => registry_schema($info->parameters_desc, true),
            'returns' => registry_schema($info->returns_desc, true),
        ];
    } catch (Throwable $error) {
        fwrite(STDERR, $record->name . ': ' . $error->getMessage() . PHP_EOL);
        exit(1);
    }
}

$document = [
    'schema_version' => 1,
    'moodle' => [
        'release' => (string)$CFG->release,
        'version' => (string)$CFG->version,
    ],
    'functions' => $functions,
];
$flags = JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE;
if (getenv('REGISTRY_COMPACT') !== '1') {
    $flags |= JSON_PRETTY_PRINT;
}
echo json_encode($document, $flags), PHP_EOL;
