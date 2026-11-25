import { describe, expect, it } from "vitest";
import pkg from "../../package.json";
import { CURRENT, compareVersions, kindOf, parseVersion, releases, shouldPopUp, unseenReleases, type Release } from "./releases";

const rel = (version: string): Release => ({ version, date: "2026-11-25", notes: [{ title: version, body: "" }] });
const versions = (list: Release[]) => list.map((r) => r.version);

describe("parseVersion", () => {
  it("reads semver and the legacy splash id", () => {
    expect(parseVersion("0.7.1")).toEqual([0, 7, 1]);
    expect(parseVersion("2026-11-22")).toEqual([0, 6, 0]);
  });
  it("rejects anything else", () => {
    for (const v of ["", "v1", "1.2", "1.2.3.4", "2026-11-23"]) expect(parseVersion(v)).toBeNull();
  });
});

describe("kindOf", () => {
  it("names major, minor and patch releases", () => {
    expect(kindOf("1.0.0")).toBe("major");
    expect(kindOf("0.8.0")).toBe("minor");
    expect(kindOf("0.1.0")).toBe("minor");
    expect(kindOf("0.7.1")).toBe("patch");
  });
});

it("compares numerically, not as text", () => {
  expect(compareVersions("0.10.0", "0.9.0")).toBeGreaterThan(0);
  expect(compareVersions("0.9.0", "0.9.0")).toBe(0);
  expect(compareVersions("0.9.0", "1.0.0")).toBeLessThan(0);
});

describe("pop-up rules", () => {
  const L = [rel("0.9.1"), rel("0.9.0"), rel("0.8.1"), rel("0.8.0")];
  it("pops up only when a newer major or minor exists", () => {
    expect(shouldPopUp("0.9.0", L)).toBe(false); // only a patch is newer
    expect(shouldPopUp("0.9.1", L)).toBe(false);
    expect(shouldPopUp("0.8.1", L)).toBe(true);
    expect(shouldPopUp("", L)).toBe(true);
    expect(shouldPopUp("garbage", L)).toBe(true);
  });
  it("lists every unseen release, patches included", () => {
    expect(versions(unseenReleases("0.8.1", L))).toEqual(["0.9.1", "0.9.0"]);
    expect(versions(unseenReleases("0.8.0", L))).toEqual(["0.9.1", "0.9.0", "0.8.1"]);
  });
  it("shows someone who never closed one only the newest feature release onward", () => {
    expect(versions(unseenReleases("", L))).toEqual(["0.9.1", "0.9.0"]);
    expect(versions(unseenReleases("garbage", L))).toEqual(["0.9.1", "0.9.0"]);
  });
});

describe("the real release list", () => {
  it("is newest first, all valid, each with notes", () => {
    for (const r of releases) {
      expect(parseVersion(r.version), r.version).not.toBeNull();
      expect(r.notes.length, r.version).toBeGreaterThan(0);
      expect(r.date).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    }
    for (let i = 1; i < releases.length; i++) expect(compareVersions(releases[i - 1].version, releases[i].version)).toBeGreaterThan(0);
  });
  it("is the app version", () => {
    expect(CURRENT).toBe(releases[0].version);
    expect(pkg.version).toBe(CURRENT);
  });
  it("shows members who closed the 22 Nov splash only what came after it", () => {
    expect(versions(unseenReleases("2026-11-22")).at(-1)).toBe("0.7.0");
    expect(shouldPopUp("2026-11-22")).toBe(true);
    expect(shouldPopUp(CURRENT)).toBe(false);
  });
});
