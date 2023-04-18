package output

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/osv-scanner/pkg/models"
	"github.com/google/osv-scanner/pkg/osv"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"golang.org/x/term"
)

// PrintTableResults prints the osv scan results into a human friendly table.
func PrintTableResults(vulnResult *models.VulnerabilityResults, outputWriter io.Writer) {
	outputTable := table.NewWriter()
	outputTable.SetOutputMirror(outputWriter)
	outputTable.AppendHeader(table.Row{"OSV URL (ID In Bold)", "Ecosystem", "Package", "Version", "Fixed Versions", "Source"})
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	isTerminal := false
	if err == nil { // If output is a terminal, set max length to width and add styling
		outputTable.SetStyle(table.StyleRounded)
		outputTable.Style().Color.Row = text.Colors{text.Reset, text.BgHiBlack}
		outputTable.Style().Color.RowAlternate = text.Colors{text.Reset, text.BgBlack}
		outputTable.Style().Options.DoNotColorBordersAndSeparators = true
		outputTable.SetAllowedRowLength(width)
		isTerminal = true
	} // Otherwise use default ascii (e.g. getting piped to a file)

	outputTable = tableBuilder(outputTable, vulnResult, isTerminal)

	if outputTable.Length() == 0 {
		return
	}
	outputTable.Render()
}

func tableBuilder(outputTable table.Writer, vulnResult *models.VulnerabilityResults, addStyling bool) table.Writer {
	rows := tableBuilderInner(vulnResult, addStyling, true)
	for _, elem := range rows {
		outputTable.AppendRow(elem.row, table.RowConfig{AutoMerge: elem.shouldMerge})
	}

	uncalledRows := tableBuilderInner(vulnResult, addStyling, false)
	if len(uncalledRows) == 0 {
		return outputTable
	}

	outputTable.AppendSeparator()
	outputTable.AppendRow(table.Row{"Uncalled vulnerabilities"})
	outputTable.AppendSeparator()

	for _, elem := range uncalledRows {
		outputTable.AppendRow(elem.row, table.RowConfig{AutoMerge: elem.shouldMerge})
	}

	return outputTable
}

type tbInnerResponse struct {
	row         table.Row
	shouldMerge bool
}

func tableBuilderInner(vulnResult *models.VulnerabilityResults, addStyling bool, calledVulns bool) []tbInnerResponse {
	allOutputRows := []tbInnerResponse{}
	// Working directory used to simplify path
	workingDir, workingDirErr := os.Getwd()
	for _, sourceRes := range vulnResult.Results {
		for _, pkg := range sourceRes.Packages {
			source := sourceRes.Source
			if workingDirErr == nil {
				sourcePath, err := filepath.Rel(workingDir, source.Path)
				if err == nil { // Simplify the path if possible
					source.Path = sourcePath
				}
			}
			vulns := pkg.Vulnerabilities
			// Merge groups into the same row
			for _, group := range pkg.Groups {
				if group.IsCalled() != calledVulns {
					continue
				}
				outputRow := table.Row{}
				shouldMerge := false
				var links []string
				for _, vuln := range group.IDs {
					if addStyling {
						links = append(links, osv.BaseVulnerabilityURL+text.Bold.EscapeSeq()+vuln+text.Reset.EscapeSeq())
					} else {
						links = append(links, osv.BaseVulnerabilityURL+vuln)
					}
				}
				outputRow = append(outputRow, strings.Join(links, "\n"))
				if pkg.Package.Ecosystem == "GIT" {
					outputRow = append(outputRow, "GIT", pkg.Package.Version, pkg.Package.Version)
					shouldMerge = true
				} else {
					outputRow = append(outputRow, pkg.Package.Ecosystem, pkg.Package.Name, pkg.Package.Version)
				}
				var fixedVersions []string
				uniqueFixedVersion := map[string]bool{}
				for _, vuln := range vulns {
					vuln_id := vuln.ID
					for _, vuln_id_from_group := range group.IDs {
						if vuln_id == vuln_id_from_group {
							for _, aff := range vuln.Affected {
								pkg_affected := aff.Package
								ecosystem_affectedPkg := fmt.Sprintf("%v", pkg_affected.Ecosystem)
								eco_frompkg := fmt.Sprintf("%v", pkg.Package.Ecosystem)
								name := pkg_affected.Name
								if pkg.Package.Name == name && eco_frompkg == ecosystem_affectedPkg {
									for _, rng := range aff.Ranges {
										rangeType := rng.Type
										evnt := rng.Events
										fixedVersion := evnt[1].Fixed
										if rangeType == "GIT" {
											fixedVersion = fixedVersion[0:10]
										}
										if _, isPresent := uniqueFixedVersion[fixedVersion]; isPresent {
											continue
										}
										uniqueFixedVersion[fixedVersion] = true
										fix := fmt.Sprintf("%v", fixedVersion)
										if addStyling {
											fixedVersions = append(fixedVersions, text.Bold.EscapeSeq()+fix+text.Reset.EscapeSeq())
										} else {
											fixedVersions = append(fixedVersions, fix)
										}
									}
								}
							}
						}
					}
				}
				outputRow = append(outputRow, strings.Join(fixedVersions, "\n"))
				outputRow = append(outputRow, source.Path)
				allOutputRows = append(allOutputRows, tbInnerResponse{
					row:         outputRow,
					shouldMerge: shouldMerge,
				})
			}
		}
	}
	return allOutputRows
}
