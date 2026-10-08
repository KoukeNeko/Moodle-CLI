//go:build !darwin

package callback

func (*Broker) prepare() error { return nil }

func (*Broker) prepareHandler() error { return nil }
