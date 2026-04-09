package canary_test

import (
	"os"
	"testing"

	"github.com/urfave/cli"
)

// TestUrfaveCLICompat is a canary for github.com/urfave/cli v1.
//
// Used in: main.go, pkg/validate/charts.go.
//
// Pins: App (NewApp, Name, Version, Usage, Commands, Run), BoolFlag, StringFlag
// (Name, Usage, Required, Destination, EnvVar, TakesFile, Value), Command
// (Name, Usage, Action, Before, Flags), Context (String method).
//
// CRITICAL: v2 is a complete rewrite with different import path (urfave/cli/v2).
// Updating to v2 would require rewriting main.go entirely.
func TestUrfaveCLICompat(t *testing.T) {
	t.Run("NewApp returns App with settable fields", func(t *testing.T) {
		// main.go:169-172
		app := cli.NewApp()
		if app == nil {
			t.Fatal("NewApp returned nil")
		}

		// Verify App fields are settable
		app.Name = "test-app"
		app.Version = "v1.0.0"
		app.Usage = "Test usage"

		if app.Name != "test-app" {
			t.Errorf("App.Name: got %q, want %q", app.Name, "test-app")
		}
		if app.Version != "v1.0.0" {
			t.Errorf("App.Version: got %q, want %q", app.Version, "v1.0.0")
		}
		if app.Usage != "Test usage" {
			t.Errorf("App.Usage: got %q, want %q", app.Usage, "Test usage")
		}
	})

	t.Run("BoolFlag with all production fields", func(t *testing.T) {
		// main.go:173-178, 213-218, etc.
		var dest bool
		flag := cli.BoolFlag{
			Name:        "test,t",
			Usage:       "Test flag",
			Required:    false,
			Destination: &dest,
		}

		// Verify fields are set correctly
		if flag.Name != "test,t" {
			t.Errorf("BoolFlag.Name: got %q, want %q", flag.Name, "test,t")
		}
		if flag.Usage != "Test flag" {
			t.Errorf("BoolFlag.Usage: got %q, want %q", flag.Usage, "Test flag")
		}
		if flag.Required != false {
			t.Errorf("BoolFlag.Required: got %v, want false", flag.Required)
		}
		if flag.Destination != &dest {
			t.Error("BoolFlag.Destination: pointer mismatch")
		}
	})

	t.Run("StringFlag with all production fields", func(t *testing.T) {
		// main.go:179-185, 186-192, etc.
		var dest string
		flag := cli.StringFlag{
			Name:        "config",
			Usage:       "Configuration file",
			TakesFile:   true,
			Destination: &dest,
			Value:       "default.yaml",
			EnvVar:      "CONFIG_FILE",
			Required:    false,
		}

		// Verify all fields are settable
		if flag.Name != "config" {
			t.Errorf("StringFlag.Name: got %q, want %q", flag.Name, "config")
		}
		if flag.Usage != "Configuration file" {
			t.Errorf("StringFlag.Usage: got %q, want %q", flag.Usage, "Configuration file")
		}
		if flag.TakesFile != true {
			t.Error("StringFlag.TakesFile: got false, want true")
		}
		if flag.Destination != &dest {
			t.Error("StringFlag.Destination: pointer mismatch")
		}
		if flag.Value != "default.yaml" {
			t.Errorf("StringFlag.Value: got %q, want %q", flag.Value, "default.yaml")
		}
		if flag.EnvVar != "CONFIG_FILE" {
			t.Errorf("StringFlag.EnvVar: got %q, want %q", flag.EnvVar, "CONFIG_FILE")
		}
		if flag.Required != false {
			t.Errorf("StringFlag.Required: got %v, want false", flag.Required)
		}
	})

	t.Run("Command struct with Action and Flags", func(t *testing.T) {
		// main.go:411-589
		cmd := cli.Command{
			Name:  "test",
			Usage: "Test command",
			Action: func(c *cli.Context) {
				// Action function for canary test
			},
			Before: func(c *cli.Context) error {
				// Before function for canary test
				return nil
			},
			Flags: []cli.Flag{
				cli.StringFlag{Name: "flag1"},
			},
		}

		// Verify Command fields
		if cmd.Name != "test" {
			t.Errorf("Command.Name: got %q, want %q", cmd.Name, "test")
		}
		if cmd.Usage != "Test command" {
			t.Errorf("Command.Usage: got %q, want %q", cmd.Usage, "Test command")
		}
		if cmd.Action == nil {
			t.Error("Command.Action is nil")
		}
		if cmd.Before == nil {
			t.Error("Command.Before is nil")
		}
		if len(cmd.Flags) != 1 {
			t.Errorf("Command.Flags: got %d flags, want 1", len(cmd.Flags))
		}
	})

	t.Run("App.Commands accepts Command slice", func(t *testing.T) {
		// main.go:411
		app := cli.NewApp()
		app.Commands = []cli.Command{
			{
				Name:  "cmd1",
				Usage: "Command 1",
			},
			{
				Name:  "cmd2",
				Usage: "Command 2",
			},
		}

		if len(app.Commands) != 2 {
			t.Errorf("App.Commands: got %d commands, want 2", len(app.Commands))
		}
	})

	t.Run("App.Run executes with args", func(t *testing.T) {
		// main.go:591
		app := cli.NewApp()
		app.Name = "canary-test"

		executed := false
		app.Commands = []cli.Command{
			{
				Name: "test-cmd",
				Action: func(c *cli.Context) error {
					executed = true
					return nil
				},
			},
		}

		// Run with test args (app name + command)
		err := app.Run([]string{"canary-test", "test-cmd"})
		if err != nil {
			t.Fatalf("App.Run: %v", err)
		}
		if !executed {
			t.Error("App.Run did not execute command action")
		}
	})

	t.Run("Context.String method reads flag value", func(t *testing.T) {
		// main.go:861, 912, 938 - c.String("flag-name")
		app := cli.NewApp()
		app.Commands = []cli.Command{
			{
				Name: "test",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name: "test-flag",
					},
				},
				Action: func(c *cli.Context) error {
					val := c.String("test-flag")
					if val != "test-value" {
						t.Errorf("Context.String: got %q, want %q", val, "test-value")
					}
					return nil
				},
			},
		}

		// Run with flag value
		if err := app.Run([]string{"app", "test", "--test-flag=test-value"}); err != nil {
			t.Fatalf("App.Run: %v", err)
		}
	})

	t.Run("EnvVar flag reads from environment", func(t *testing.T) {
		// main.go:191, 204, 211, etc. - EnvVar field usage
		const envKey = "CANARY_TEST_FLAG"
		const envValue = "from-env"

		os.Setenv(envKey, envValue)
		defer os.Unsetenv(envKey)

		var dest string
		app := cli.NewApp()
		app.Flags = []cli.Flag{
			cli.StringFlag{
				Name:        "test",
				EnvVar:      envKey,
				Destination: &dest,
			},
		}
		app.Action = func(c *cli.Context) error {
			if dest != envValue {
				t.Errorf("EnvVar: Destination got %q, want %q", dest, envValue)
			}
			if c.String("test") != envValue {
				t.Errorf("EnvVar: Context.String got %q, want %q", c.String("test"), envValue)
			}
			return nil
		}

		if err := app.Run([]string{"app"}); err != nil {
			t.Fatalf("App.Run: %v", err)
		}
	})

	t.Run("Flag Destination updates variable", func(t *testing.T) {
		// main.go:177, 185, 190, etc. - Destination field usage
		var boolDest bool
		var stringDest string

		app := cli.NewApp()
		app.Flags = []cli.Flag{
			cli.BoolFlag{
				Name:        "bool-flag",
				Destination: &boolDest,
			},
			cli.StringFlag{
				Name:        "string-flag",
				Destination: &stringDest,
			},
		}
		app.Action = func(c *cli.Context) error {
			if !boolDest {
				t.Error("BoolFlag Destination: got false, want true")
			}
			if stringDest != "value" {
				t.Errorf("StringFlag Destination: got %q, want %q", stringDest, "value")
			}
			return nil
		}

		if err := app.Run([]string{"app", "--bool-flag", "--string-flag=value"}); err != nil {
			t.Fatalf("App.Run: %v", err)
		}
	})
}
