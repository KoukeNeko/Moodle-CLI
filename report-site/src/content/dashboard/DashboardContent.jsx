import React from "react";
import {
  DataComponent,
  DataTable,
  Filters,
  MetricCard,
  Section,
  useDataApp,
  useDashboardTabs,
} from "../../data-app-public.jsx";
import "./dashboard.css";

const tabs = [
  { id: "overview", label: "Overview", filterIds: ["version"] },
  { id: "functions", label: "Function coverage", filterIds: ["version", "component", "effect"] },
  { id: "roles", label: "Role matrix", filterIds: ["version", "status"] },
  { id: "scale", label: "Scale", filterIds: ["version", "status"] },
  { id: "runs", label: "Runs", filterIds: [] },
];

const number = value => value == null ? "—" : new Intl.NumberFormat("en-US").format(value);
const seconds = value => value == null ? "—" : `${Number(value).toFixed(2)} s`;
const bytes = value => {
  if (value == null) return "—";
  const units = ["B", "KiB", "MiB", "GiB"];
  let amount = Number(value), unit = 0;
  while (amount >= 1024 && unit < units.length - 1) { amount /= 1024; unit += 1; }
  return `${amount.toFixed(unit < 2 ? 0 : 1)} ${units[unit]}`;
};

function EvidenceTable({ id, queryId, title, description, rows, columns, searchable = true }) {
  return <DataComponent variant="card" id={id} queryId={queryId} title={title}
    description={description} kind="table" displayRows={rows} sourceRows={rows}>
    <DataTable rows={rows} columns={columns} searchable={searchable} label={title} />
  </DataComponent>;
}

export function DashboardContent() {
  const { snapshot, queries, filters, setFilter, reviewedRows } = useDataApp();
  const { activeTabId } = useDashboardTabs(tabs);
  const tab = tabs.some(item => item.id === activeTabId) ? activeTabId : "overview";
  const versions = reviewedRows("versions", ["version"]);
  const functions = reviewedRows("functions", ["version", "component", "effect", "function"]);
  const roles = reviewedRows("roles", ["version", "role", "status"]);
  const scale = reviewedRows("scale", ["version", "status"]);
  const runs = reviewedRows("runs", ["generated_at"]);
  const latestRun = runs.at(-1) ?? {};
  const latestScale = scale.at(-1) ?? {};
  const registryTotal = latestRun.registry_functions ?? new Set(functions.map(row => row.function)).size;
  const executedRoleCells = roles.filter(row => row.status !== "not-run").length;

  return <article className="verification-dashboard">
    <Filters sticky filters={snapshot.filters ?? []} queries={queries} values={filters} onChange={setFilter} />

    {tab === "overview" && <>
      <div className="status-note" role="note">
        <span className="status-note__marker">Observed evidence</span>
        <span>Registry coverage and scale acceptance are separate claims. The current snapshot records the
          role/function matrix as <strong>not run</strong>; no missing execution is reported as a pass or skip.</span>
      </div>
      <Section id="overview-metrics-title" title="Verification snapshot" spacing="none">
        <div className="metric-grid">
          <MetricCard id="registry-size" title="Core function union" queryId="runs"
            sourceRows={runs} value={number(registryTotal)} description="Distinct core external functions across Moodle 4.5, 5.1, and 5.2 registry snapshots." />
          <MetricCard id="version-coverage" title="Moodle versions" queryId="versions"
            sourceRows={versions} value={number(versions.length)} description="Version-specific registries generated from disposable official Moodle environments." />
          <MetricCard id="scale-students" title="Synthetic students" queryId="scale"
            sourceRows={scale} value={number(latestScale.students)} description="Students in the deterministic ten-year PostgreSQL scale fixture." />
          <MetricCard id="scale-enrolments" title="Enrolment facts" queryId="scale"
            sourceRows={scale} value={number(latestScale.enrolments)} description="Academic and orientation enrolment relations validated independently with SQL." />
          <MetricCard id="role-summary" title="Executed role cells" queryId="roles"
            sourceRows={roles} value={number(executedRoleCells)} description="Role/function cells backed by an execution artifact in this snapshot." />
        </div>
      </Section>
      <Section id="overview-versions-title" title="Registry inventory" spacing="after-metrics">
        <EvidenceTable id="overview-version-table" queryId="versions" title="Version coverage"
          description="Read and write classification is generated per Moodle release, not inferred from command names."
          rows={versions} searchable={false} columns={[
            { field: "version", label: "Target" }, { field: "release", label: "Moodle release" },
            { field: "functions", label: "Functions" }, { field: "read", label: "Read" },
            { field: "write", label: "Write" }, { field: "deprecated", label: "Deprecated" },
          ]} />
      </Section>
    </>}

    {tab === "functions" && <Section id="function-coverage-title" title="Core external functions" spacing="none">
      <EvidenceTable id="function-table" queryId="functions" title="Registry coverage"
        description="Every row is present in a generated version registry. Registry-covered does not claim that every role has executed the function."
        rows={functions} columns={[
          { field: "version", label: "Version" }, { field: "function", label: "Function" },
          { field: "component", label: "Component" }, { field: "effect", label: "Effect", presentation: "status" },
          { field: "destructive", label: "Destructive" }, { field: "transport", label: "Transport" },
          { field: "recipe", label: "Recipe" }, { field: "result", label: "Coverage", presentation: "status" },
        ]} />
    </Section>}

    {tab === "roles" && <>
      <div className="status-note status-note--warning" role="note">
        <span>The runtime role/function harness has not emitted an artifact for this snapshot. Cells remain
          <strong> not-run</strong> until Moodle actually returns passed, expected denied, expected unavailable, or failed.</span>
      </div>
      <Section id="role-matrix-title" title="Role execution matrix" spacing="none">
        <EvidenceTable id="role-table" queryId="roles" title="Role and domain results"
          description="Includes built-in, administrator, archetype-based custom, and no-archetype custom identities."
          rows={roles} columns={[
            { field: "version", label: "Version" }, { field: "role", label: "Identity" },
            { field: "domain", label: "Domain" }, { field: "passed", label: "Passed" },
            { field: "expected_denied", label: "Expected denied" },
            { field: "expected_unavailable", label: "Expected unavailable" },
            { field: "failed", label: "Failed" }, { field: "status", label: "Status", presentation: "status" },
          ]} />
      </Section>
    </>}

    {tab === "scale" && <Section id="scale-results-title" title="Ten-year scale acceptance" spacing="none">
      <div className="metric-grid metric-grid--scale">
        <MetricCard id="scale-latency" title="Participant page p95" queryId="scale" sourceRows={scale}
          value={seconds(latestScale.participant_p95_seconds)} description="Wall time across the 50 pages used to enumerate the 50,000-participant course." />
        <MetricCard id="scale-memory" title="Peak CLI RSS" queryId="scale" sourceRows={scale}
          value={bytes(latestScale.peak_rss_bytes)} description="Maximum resident set recorded for participant-page CLI calls." />
        <MetricCard id="scale-http" title="REST requests" queryId="scale" sourceRows={scale}
          value={number(latestScale.http_requests)} description={latestScale.http_request_budget == null
            ? "REST-only request count; an em dash means the corrected request gate has not been rerun."
            : `REST-only request count during pagination; linear budget ${number(latestScale.http_request_budget)}.`} />
      </div>
      <EvidenceTable id="scale-table" queryId="scale" title="Scale truth and performance"
        description="Control-plane truth is calculated directly from PostgreSQL; the CLI remains the system under test."
        rows={scale} searchable={false} columns={[
          { field: "version", label: "Version" }, { field: "status", label: "Status", presentation: "status" },
          { field: "students", label: "Students" }, { field: "courses", label: "Courses" },
          { field: "enrolments", label: "Enrolments" }, { field: "span_days", label: "Span days" },
          { field: "undergraduate_credits", label: "UG credits / term" },
          { field: "graduate_credits", label: "Graduate credits / term" },
          { field: "participant_pages", label: "Pages" },
          { field: "http_requests", label: "REST requests" },
          { field: "http_request_budget", label: "Request budget" },
        ]} />
    </Section>}

    {tab === "runs" && <Section id="runs-title" title="Run provenance" spacing="none">
      <EvidenceTable id="runs-table" queryId="runs" title="Published snapshots"
        description="Commit and runner metadata make dashboard claims traceable to a reproducible run. Secrets and request payloads are excluded."
        rows={runs} searchable={false} columns={[
          { field: "run", label: "Run" }, { field: "commit", label: "Commit" },
          { field: "generated_at", label: "Generated at" }, { field: "runner", label: "Runner" },
          { field: "runner_image", label: "Runner image" },
          { field: "registry_functions", label: "Registry union" },
          { field: "role_matrix", label: "Role matrix", presentation: "status" },
          { field: "scale", label: "Scale", presentation: "status" },
        ]} />
    </Section>}
  </article>;
}
