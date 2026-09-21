// SportSelectorValue mirrors fitter.v1.SportSelector's oneof for editing:
// `case: undefined` means "all sports" (the wire message is entirely unset
// in that case, per common.proto's doc comment).
export type SportSelectorValue =
  | { case: undefined }
  | { case: "group"; value: string }
  | { case: "sportType"; value: string };

export function allSports(): SportSelectorValue {
  return { case: undefined };
}

// sportSelectorInit converts an edited value into the shape a
// CreateGoalRequest/UpdateGoalRequest's `sport` field expects — undefined
// for "all sports", otherwise the oneof wrapper.
export function sportSelectorInit(v: SportSelectorValue): { selector: SportSelectorValue } | undefined {
  if (v.case === undefined) return undefined;
  return { selector: v };
}
