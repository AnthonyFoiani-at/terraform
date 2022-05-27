package e2etest

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform/internal/e2e"
)

// The tests in this file are for the "terraform providers mirror" command,
// which is tested in an e2etest mode rather than a unit test mode because it
// interacts directly with Terraform Registry and the full details of that are
// tricky to mock. Such a mock is _possible_, but we're using e2etest as a
// compromise for now to keep these tests relatively simple.

func findFiles(outputDir string) (got []string, err error) {
	err = filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil // we only care about leaf files for this test
		}
		relPath, err := filepath.Rel(outputDir, path)
		if err != nil {
			return err
		}
		got = append(got, filepath.ToSlash(relPath))
		return nil
	})
	sort.Strings(got)
	return
}

func TestTerraformProvidersMirror(t *testing.T) {
	// This test reaches out to releases.hashicorp.com to download the
	// template and null providers, so it can only run if network access is
	// allowed.
	skipIfCannotAccessNetwork(t)

	outputDir := t.TempDir()
	t.Logf("creating mirror directory in %s", outputDir)

	fixturePath := filepath.Join("testdata", "terraform-providers-mirror")
	tf := e2e.NewBinary(t, terraformBin, fixturePath)

	stdout, stderr, err := tf.Run("providers", "mirror", "-platform=linux_amd64", "-platform=windows_386", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %s\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	// The test fixture includes exact version constraints for the two
	// providers it depends on so that the following should remain stable.
	// In the (unlikely) event that these particular versions of these
	// providers are removed from the registry, this test will start to fail.
	want := []string{
		"registry.terraform.io/hashicorp/null/2.1.0.json",
		"registry.terraform.io/hashicorp/null/index.json",
		"registry.terraform.io/hashicorp/null/terraform-provider-null_2.1.0_linux_amd64.zip",
		"registry.terraform.io/hashicorp/null/terraform-provider-null_2.1.0_windows_386.zip",
		"registry.terraform.io/hashicorp/template/2.1.1.json",
		"registry.terraform.io/hashicorp/template/index.json",
		"registry.terraform.io/hashicorp/template/terraform-provider-template_2.1.1_linux_amd64.zip",
		"registry.terraform.io/hashicorp/template/terraform-provider-template_2.1.1_windows_386.zip",
	}
	got, err := findFiles(outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected files in result\n%s", diff)
	}
}

func TestTerraformProvidersMirrorKeep(t *testing.T) {
	// Mostly a brute-force copy of `TestTerraformProvidersMirror` above. Maybe
	// refactor eventually?

	// This test reaches out to releases.hashicorp.com to download the
	// template and null providers, so it can only run if network access is
	// allowed.
	skipIfCannotAccessNetwork(t)

	outputDir := t.TempDir()
	t.Logf("creating mirror directory in %s", outputDir)

	fixturePath := filepath.Join("testdata", "terraform-providers-mirror")
	tf := e2e.NewBinary(t, terraformBin, fixturePath)

	stdout, stderr, err := tf.Run("providers", "mirror", "-platform=linux_amd64", "-platform=windows_386", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %s\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	// Delete some of the .zip files:
	victim1Path := filepath.Join(outputDir, "registry.terraform.io/hashicorp/null/terraform-provider-null_2.1.0_linux_amd64.zip")
	err = os.Remove(victim1Path)
	if err != nil {
		t.Fatal(err)
	}
	victim2Path := filepath.Join(outputDir, "registry.terraform.io/hashicorp/template/terraform-provider-template_2.1.1_windows_386.zip")
	err = os.Remove(victim2Path)
	if err != nil {
		t.Fatal(err)
	}

	// Grab the last-modified time on the others (to make sure they aren't overwritten).
	keeper1Path := filepath.Join(outputDir, "registry.terraform.io/hashicorp/null/terraform-provider-null_2.1.0_windows_386.zip")
	keeper1StatPre, err := os.Stat(keeper1Path)
	if err != nil {
		t.Fatal(err)
	}
	keeper2Path := filepath.Join(outputDir, "registry.terraform.io/hashicorp/template/terraform-provider-template_2.1.1_linux_amd64.zip")
	keeper2StatPre, err := os.Stat(keeper2Path)
	if err != nil {
		t.Fatal(err)
	}

	// Pause to ensure that our mtimes are incremented by at least 1 second.
	time.Sleep(1)

	// Now run the command again, with the `-keep` flag set:
	stdout, stderr, err = tf.Run("providers", "mirror", "-keep", "-platform=linux_amd64", "-platform=windows_386", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %s\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	// Do same test as original, in case `-keep` broke fetching entirely.

	// The test fixture includes exact version constraints for the two
	// providers it depends on so that the following should remain stable.
	// In the (unlikely) event that these particular versions of these
	// providers are removed from the registry, this test will start to fail.
	want := []string{
		"registry.terraform.io/hashicorp/null/2.1.0.json",
		"registry.terraform.io/hashicorp/null/index.json",
		"registry.terraform.io/hashicorp/null/terraform-provider-null_2.1.0_linux_amd64.zip",
		"registry.terraform.io/hashicorp/null/terraform-provider-null_2.1.0_windows_386.zip",
		"registry.terraform.io/hashicorp/template/2.1.1.json",
		"registry.terraform.io/hashicorp/template/index.json",
		"registry.terraform.io/hashicorp/template/terraform-provider-template_2.1.1_linux_amd64.zip",
		"registry.terraform.io/hashicorp/template/terraform-provider-template_2.1.1_windows_386.zip",
	}
	got, err := findFiles(outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected files in result\n%s", diff)
	}

	// Now also check that the "keepers" didn't get their mtimes changed:
	keeper1StatPost, err := os.Stat(keeper1Path)
	if err != nil {
		t.Fatal(err)
	}
	if keeper1StatPre.ModTime() != keeper1StatPost.ModTime() {
		t.Errorf("mtime changed for %v", keeper1Path)
	}

	keeper2StatPost, err := os.Stat(keeper2Path)
	if err != nil {
		t.Fatal(err)
	}
	if keeper2StatPre.ModTime() != keeper2StatPost.ModTime() {
		t.Errorf("mtime changed for %v", keeper2Path)
	}
}
