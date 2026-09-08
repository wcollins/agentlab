package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gridctl/gridctl/pkg/config"
	"github.com/gridctl/gridctl/pkg/skills"
	"github.com/gridctl/gridctl/pkg/state"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	exportOutputDir string
	exportFormat    string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the running deployment's authored configuration",
	Long: `Reread the stack configuration associated with the running deployment.
Export preserves authored variable expressions rather than effective runtime values.
Recognized inline credentials block export. Review other authored literals before
sharing; arbitrary commands, encoded content, and URL queries are not secret-scanned.
This is a semantic export, not a byte-for-byte copy. Recipients may need variables
and the same relative file layout. YAML directory exports may include skills.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runExportContext(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutputDir, "output", "o", "", "Output directory (default: stdout)")
	exportCmd.Flags().StringVar(&exportFormat, "format", "yaml", "Output format: yaml or json")
}

func runExportContext(ctx context.Context, stdout, stderr io.Writer) error {
	states, err := state.List()
	if err != nil {
		return fmt.Errorf("listing running stacks: %w", err)
	}
	for i := range states {
		if state.IsRunning(&states[i]) {
			return exportStackFile(ctx, states[i].StackFile, stdout, stderr)
		}
	}
	return fmt.Errorf("no running stack found")
}

func exportStackFile(ctx context.Context, path string, stdout, stderr io.Writer) error {
	if exportFormat != "yaml" && exportFormat != "json" {
		return fmt.Errorf("export format must be yaml or json")
	}
	stack, sources, err := config.ExportStack(ctx, path)
	if err != nil {
		return err
	}
	var data []byte
	if exportFormat == "json" {
		data, err = json.MarshalIndent(stack, "", "  ")
	} else {
		data, err = yaml.Marshal(stack)
	}
	if err != nil {
		return fmt.Errorf("export: cannot encode stack")
	}
	if exportOutputDir == "" {
		if _, err := fmt.Fprintln(stderr, config.ExportNotice); err != nil {
			return err
		}
		_, err = stdout.Write(data)
		return err
	}
	artifacts := []exportArtifact{{"stack." + exportFormat, data}}
	if exportFormat == "yaml" {
		sidecar, err := exportSkillsData(ctx)
		if err != nil {
			return err
		}
		if sidecar != nil {
			artifacts = append(artifacts, exportArtifact{"skills.yaml", sidecar})
		}
	}
	return writeExportArtifacts(ctx, exportOutputDir, sources, artifacts, stderr)
}

type exportArtifact struct {
	name string
	data []byte
}

func writeExportArtifacts(ctx context.Context, dir string, sources []string, artifacts []exportArtifact, stderr io.Writer) error {
	for _, artifact := range artifacts {
		target := filepath.Join(dir, artifact.name)
		info, err := os.Stat(target)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("export: cannot inspect output destination")
		}
		for _, source := range sources {
			sourceInfo, err := os.Stat(source)
			if err != nil {
				return fmt.Errorf("export: cannot verify source protection")
			}
			if info != nil && os.SameFile(info, sourceInfo) {
				return fmt.Errorf("export: output would overwrite a source stack or ancestor; choose another directory")
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("export: cannot create output directory; no artifacts written")
	}
	written := []string{}
	for _, artifact := range artifacts {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("export cancelled; already written: %v: %w", written, err)
		}
		if err := os.WriteFile(filepath.Join(dir, artifact.name), artifact.data, 0644); err != nil {
			return fmt.Errorf("export: writing %s failed (it may be incomplete); already written: %v", artifact.name, written)
		}
		written = append(written, artifact.name)
	}
	_, err := fmt.Fprintf(stderr, "%s\nWrote %v\n", config.ExportNotice, written)
	return err
}

func exportSkillsData(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	lf, err := skills.ReadLockFile(skills.LockFilePath())
	if err != nil {
		return nil, fmt.Errorf("export: cannot read skills metadata")
	}
	if len(lf.Sources) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(lf.Sources))
	for name := range lf.Sources {
		names = append(names, name)
	}
	sort.Strings(names)
	var out struct {
		Sources []skillSourceYAML `yaml:"sources"`
	}
	for i, name := range names {
		src := lf.Sources[name]
		u, err := url.Parse(src.Repo)
		if (err != nil && strings.Contains(src.Repo, "://")) || (err == nil && u.User != nil && u.User.String() != "") {
			return nil, fmt.Errorf("export: skills.sources[%d].repo: invalid URL or credential-bearing userinfo; remove userinfo before exporting", i)
		}
		out.Sources = append(out.Sources, skillSourceYAML{Name: name, Repo: src.Repo, Ref: src.Ref})
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("export: cannot encode skills metadata")
	}
	return data, nil
}

type skillSourceYAML struct {
	Name string `yaml:"name"`
	Repo string `yaml:"repo"`
	Ref  string `yaml:"ref,omitempty"`
}
