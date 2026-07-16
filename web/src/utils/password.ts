export const MIN_PASSWORD_CHARACTERS = 8;
export const MAX_PASSWORD_UTF8_BYTES = 72;

export type NewPasswordValidationError = "too_long" | "too_short";

export const validateNewPassword = (value: string): NewPasswordValidationError | undefined => {
  if (Array.from(value).length < MIN_PASSWORD_CHARACTERS) return "too_short";
  if (new TextEncoder().encode(value).length > MAX_PASSWORD_UTF8_BYTES) return "too_long";
  return undefined;
};
