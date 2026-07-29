package analysisbench

import (
	"fmt"
	"strings"
)

func Markdown(report *Report) (string, error) {
	if report == nil || len(report.Candidates) == 0 {
		return "", fmt.Errorf("benchmark report has no candidates")
	}
	var output strings.Builder
	fmt.Fprintf(&output, "# Audio analysis benchmark: %s / %s\n\n", markdownText(report.ManifestID), markdownText(report.ManifestRevision))
	fmt.Fprintf(&output, "Manifest SHA-256: `%s`\n\n", report.ManifestSHA256)
	fmt.Fprintf(&output, "Calibration samples: %d; held-out evaluation samples used for every reported metric: %d.\n\n",
		report.CalibrationSamples, report.EvaluationSamples)
	output.WriteString("| Candidate | Macro-F1 | High-confidence precision | Coverage | Abstention | Failures | Mean / p95 CPU ms | Peak memory MiB | Image delta MiB |\n")
	output.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, candidate := range report.Candidates {
		fmt.Fprintf(&output, "| %s | %.1f%% | %.1f%% (%d) | %.1f%% | %.1f%% | %.1f%% | %.2f / %.2f | %.1f | %.1f |\n",
			markdownText(candidate.Candidate.ID), percentage(candidate.MacroF1), percentage(candidate.HighConfidencePrecision),
			candidate.HighConfidenceCount, percentage(candidate.Coverage), percentage(candidate.AbstentionRate),
			percentage(candidate.FailureRate), candidate.MeanCPUTimeMS, candidate.P95CPUTimeMS,
			mebibytes(candidate.PeakMemoryBytes), mebibytes(candidate.ImageDeltaBytes),
		)
	}
	for _, candidate := range report.Candidates {
		fmt.Fprintf(&output, "\n## %s\n\n", markdownText(candidate.Candidate.ID))
		fmt.Fprintf(&output, "Implementation `%s`, model `%s` (`%s`); code license: %s; model license: %s.\n\n",
			markdownText(candidate.Candidate.ImplementationVersion), markdownText(candidate.Candidate.ModelVersion),
			markdownText(candidate.Candidate.ModelDigest),
			markdownText(candidate.Candidate.CodeLicense), markdownText(candidate.Candidate.ModelLicense),
		)
		output.WriteString("| Preset | Support | Predicted | Precision | Recall | F1 |\n")
		output.WriteString("| --- | ---: | ---: | ---: | ---: | ---: |\n")
		for _, preset := range presetIDs {
			metrics := candidate.PerClass[preset]
			fmt.Fprintf(&output, "| %s | %d | %d | %.1f%% | %.1f%% | %.1f%% |\n",
				preset, metrics.Support, metrics.Predicted, percentage(metrics.Precision), percentage(metrics.Recall), percentage(metrics.F1))
		}
		output.WriteString("\nConfusion matrix (rows are expected; columns are predicted):\n\n")
		columns := append(presetIDSlice(), PredictionAbstained)
		output.WriteString("| Expected \\ Predicted |")
		for _, column := range columns {
			fmt.Fprintf(&output, " %s |", column)
		}
		output.WriteString("\n| --- |")
		for range columns {
			output.WriteString(" ---: |")
		}
		output.WriteByte('\n')
		for _, row := range append(presetIDSlice(), ExpectedNone) {
			fmt.Fprintf(&output, "| %s |", row)
			for _, column := range columns {
				fmt.Fprintf(&output, " %d |", candidate.ConfusionMatrix[row][column])
			}
			output.WriteByte('\n')
		}
	}
	return output.String(), nil
}

func percentage(value float64) float64 { return value * 100 }

func mebibytes(value int64) float64 { return float64(value) / (1024 * 1024) }

func markdownText(value string) string {
	replacer := strings.NewReplacer("|", "\\|", "\r", " ", "\n", " ")
	return replacer.Replace(value)
}
