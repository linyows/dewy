package main

import (
	"os"
	"time"

	starter "github.com/lestrrat-go/server-starter"
)

// StarterConfig struct
type StarterConfig struct {
	args       []string
	command    string
	dir        string
	interval   int
	pidfile    string
	ports      []string
	paths      []string
	sigonhup   string
	sigonterm  string
	statusfile string
}

// Args for StarterConfig
func (c StarterConfig) Args() []string { return c.args }

// Command for StarterConfig
func (c StarterConfig) Command() string { return c.command }

// Dir for StarterConfig
func (c StarterConfig) Dir() string { return c.dir }

// Interval for StarterConfig
func (c StarterConfig) Interval() time.Duration { return time.Duration(c.interval) * time.Second }

// PidFile for StarterConfig
func (c StarterConfig) PidFile() string { return c.pidfile }

// Ports for StarterConfig
func (c StarterConfig) Ports() []string { return c.ports }

// Paths for StarterConfig
func (c StarterConfig) Paths() []string { return c.paths }

// SignalOnHUP for StarterConfig
func (c StarterConfig) SignalOnHUP() os.Signal { return starter.SigFromName(c.sigonhup) }

// SignalOnTERM for StarterConfig
func (c StarterConfig) SignalOnTERM() os.Signal { return starter.SigFromName(c.sigonterm) }

// StatusFile for StarterConfig
func (c StarterConfig) StatusFile() string { return c.statusfile }
