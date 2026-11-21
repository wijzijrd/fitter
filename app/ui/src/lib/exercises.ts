// Suggested exercises for the lifts form and the max-weight picker; any other
// name works too. The server folds names to this lower-case form.
export const exercises = ["bench press", "squat", "deadlift", "overhead press", "pull-up"];

export const normalizeExercise = (s: string) => s.trim().toLowerCase().replace(/\s+/g, " ");
