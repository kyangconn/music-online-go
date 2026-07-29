import type { PresetID } from "@/types/api";

export const presetIDs: readonly PresetID[] = [
  "calm_flow",
  "kinetic_pulse",
  "cosmic_drift",
  "bass_impact",
];

export const isPresetID = (value: unknown): value is PresetID =>
  typeof value === "string" && presetIDs.some((preset) => preset === value);

export const presetTagType = (preset: PresetID | "" | undefined) => {
  if (preset === "calm_flow") return "success";
  if (preset === "kinetic_pulse") return "warning";
  if (preset === "bass_impact") return "danger";
  return "info";
};
