export const passwordLengthMessage = "Password must be at least 8 characters";
export const passwordPolicyMessage =
  "Password must contain at least one uppercase letter, one lowercase letter, one number, and one special character";

export function satisfiesPasswordPolicy(password: string): boolean {
  return (
    /[a-z]/.test(password) &&
    /[A-Z]/.test(password) &&
    /[0-9]/.test(password) &&
    /[^A-Za-z0-9\s]/.test(password)
  );
}
