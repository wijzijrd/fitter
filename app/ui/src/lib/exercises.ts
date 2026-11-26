// The server folds exercise names to this lower-case key (service.normalizeExercise); goal
// clauses refer to an exercise by it.
export const normalizeExercise = (s: string) => s.trim().toLowerCase().replace(/\s+/g, " ");
