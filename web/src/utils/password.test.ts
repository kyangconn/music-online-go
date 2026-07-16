import { describe, expect, it } from "vitest";
import { validateNewPassword } from "./password";

describe("validateNewPassword", () => {
  it.each([
    ["seven characters", "1234567", "too_short"],
    ["eight Unicode characters", "密码密码密码密码", undefined],
    ["72 ASCII bytes", "a".repeat(72), undefined],
    ["73 ASCII bytes", "a".repeat(73), "too_long"],
    ["72 multibyte UTF-8 bytes", "密".repeat(24), undefined],
    ["75 multibyte UTF-8 bytes", "密".repeat(25), "too_long"],
  ])("validates %s", (_name, value, expected) => {
    expect(validateNewPassword(value)).toBe(expected);
  });
});
