package runner

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

func hasExplicitBackend(workingDir, repoDir, toolName string) (bool, error) {
	parser := hclparse.NewParser()
	tfFiles, err := filepath.Glob(filepath.Join(workingDir, "*.tf"))
	if err != nil {
		return false, fmt.Errorf("list Terraform configuration files: %w", err)
	}
	jsonFiles, err := filepath.Glob(filepath.Join(workingDir, "*.tf.json"))
	if err != nil {
		return false, fmt.Errorf("list Terraform JSON configuration files: %w", err)
	}
	for _, path := range append(tfFiles, jsonFiles...) {
		var file *hcl.File
		var diags hcl.Diagnostics
		if strings.HasSuffix(path, ".json") {
			file, diags = parser.ParseJSONFile(path)
		} else {
			file, diags = parser.ParseHCLFile(path)
		}
		if diags.HasErrors() {
			return false, fmt.Errorf("parse Terraform configuration %s: %s", path, diags.Error())
		}
		content, _, diags := file.Body.PartialContent(&hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{{Type: "terraform"}}})
		if diags.HasErrors() {
			return false, fmt.Errorf("read Terraform configuration %s: %s", path, diags.Error())
		}
		for _, terraform := range content.Blocks {
			backendContent, _, diags := terraform.Body.PartialContent(&hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{
				{Type: "backend", LabelNames: []string{"type"}},
				{Type: "cloud"},
			}})
			if diags.HasErrors() {
				return false, fmt.Errorf("read Terraform backend configuration %s: %s", path, diags.Error())
			}
			if len(backendContent.Blocks) > 0 {
				return true, nil
			}
		}
	}
	if toolName != "terragrunt" {
		return false, nil
	}
	for dir := workingDir; ; dir = filepath.Dir(dir) {
		files, err := filepath.Glob(filepath.Join(dir, "*.hcl"))
		if err != nil {
			return false, fmt.Errorf("list Terragrunt configuration files: %w", err)
		}
		for _, path := range files {
			file, diags := parser.ParseHCLFile(path)
			if diags.HasErrors() {
				return false, fmt.Errorf("parse Terragrunt configuration %s: %s", path, diags.Error())
			}
			content, _, diags := file.Body.PartialContent(&hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{
				{Type: "remote_state"},
				{Type: "generate", LabelNames: []string{"name"}},
			}})
			if diags.HasErrors() {
				return false, fmt.Errorf("read Terragrunt configuration %s: %s", path, diags.Error())
			}
			for _, block := range content.Blocks {
				if block.Type == "remote_state" || block.Type == "generate" && len(block.Labels) > 0 && strings.Contains(strings.ToLower(block.Labels[0]), "backend") {
					return true, nil
				}
			}
		}
		if dir == repoDir || filepath.Dir(dir) == dir {
			break
		}
	}
	return false, nil
}
