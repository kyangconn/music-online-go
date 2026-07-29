import { describe, expect, it } from "vitest";
import { isPresetID, presetIDs } from "@/utils/presets";

describe("preset identifiers", () => {
  it("keeps the four API identifiers stable and rejects route noise", () => {
    expect(presetIDs).toEqual(["calm_flow", "kinetic_pulse", "cosmic_drift", "bass_impact"]);
    expect(isPresetID("cosmic_drift")).toBe(true);
    expect(isPresetID("Cosmic Drift")).toBe(false);
    expect(isPresetID(["calm_flow"])).toBe(false);
  });
});
