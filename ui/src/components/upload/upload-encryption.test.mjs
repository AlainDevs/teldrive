import { describe, expect, test } from "bun:test";
import { resolveUploadEncryption } from "./encryption";

describe("resolveUploadEncryption", () => {
  test("uses explicit share policy over local setting", () => {
    expect(resolveUploadEncryption(true, false)).toBe(true);
    expect(resolveUploadEncryption(false, true)).toBe(false);
  });

  test("falls back to authenticated drive local setting", () => {
    expect(resolveUploadEncryption(undefined, true)).toBe(true);
    expect(resolveUploadEncryption(undefined, false)).toBe(false);
  });
});
