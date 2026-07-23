import { describe, expect, test } from "bun:test";

import { deleteConfirmation, deleteTargets } from "./delete-files.ts";

const file = (id, name, isDir = false) => ({ id, isDir, name });

describe("deleteTargets", () => {
  test("uses context-menu action targets when global selection is empty", () => {
    const target = file("file-1", "inside.txt");

    expect(deleteTargets([target])).toEqual([target]);
  });

  test("preserves every action-scoped toolbar target", () => {
    const targets = [file("file-1", "one.txt"), file("file-2", "two.txt")];

    expect(deleteTargets(targets)).toEqual(targets);
  });
});

describe("deleteConfirmation", () => {
  test("counts selected files", () => {
    expect(deleteConfirmation([file("file-1", "one.txt")])).toBe(
      "Are you sure you want to delete 1 file?",
    );
  });

  test("describes recursive folder deletion without claiming zero files", () => {
    expect(deleteConfirmation([file("folder-1", "Core", true)])).toBe(
      'Are you sure you want to delete folder "Core" and all its contents?',
    );
  });
});
