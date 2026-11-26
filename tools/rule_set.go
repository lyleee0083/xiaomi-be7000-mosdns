package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/IrineSistiana/mosdns/v5/pkg/ruleset"
	"github.com/spf13/cobra"
)

func newRuleSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rule-set",
		Short: "Build MosDNS binary rule files.",
	}
	var output string
	compile := &cobra.Command{
		Use:   "compile source.json",
		Short: "Compile a JSON domain or IP rule set.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return compileRuleSet(args[0], output)
		},
	}
	compile.Flags().StringVarP(&output, "output", "o", "", "output file (default: source name with .mdrs extension)")
	cmd.AddCommand(compile)
	return cmd
}

func compileRuleSet(input, output string) error {
	ext := filepath.Ext(input)
	if !strings.EqualFold(ext, ".json") {
		return errors.New("rule source must be a .json file")
	}
	if output == "" {
		output = strings.TrimSuffix(input, ext) + ruleset.Extension
	}
	source, err := os.Open(input)
	if err != nil {
		return err
	}
	defer source.Close()
	existing, err := os.Stat(output)
	if err == nil {
		inputInfo, err := source.Stat()
		if err != nil {
			return err
		}
		if os.SameFile(inputInfo, existing) {
			return errors.New("the output and source refer to the same file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	data, err := ruleset.Compile(source)
	if err != nil {
		return fmt.Errorf("compile %s: %w", input, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(output), ".mosdns-rules-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp.Name(), output)
}
