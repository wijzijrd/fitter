-- +goose Up
-- Seed the library, then fold existing activity_lifts into blocks. Both parts are idempotent.

-- key = lower(name): every seed name below is already single-spaced.
INSERT INTO exercises (id, name, key, measure, loaded, muscles, created_by, created_at)
SELECT gen_random_uuid()::TEXT, s.name, lower(s.name), s.measure, s.loaded, s.muscles, NULL, extract(epoch FROM now())::BIGINT
FROM (VALUES
    -- weight × reps
    ('Squat',                  'weight_reps', false, ARRAY['quads','glutes']),
    ('Front squat',            'weight_reps', false, ARRAY['quads','core']),
    ('Deadlift',               'weight_reps', false, ARRAY['back','glutes','hamstrings']),
    ('Romanian deadlift',      'weight_reps', false, ARRAY['hamstrings','glutes']),
    ('Bench press',            'weight_reps', false, ARRAY['chest','triceps','shoulders']),
    ('Incline bench press',    'weight_reps', false, ARRAY['chest','shoulders','triceps']),
    ('Overhead press',         'weight_reps', false, ARRAY['shoulders','triceps']),
    ('Push press',             'weight_reps', false, ARRAY['shoulders','triceps']),
    ('Barbell row',            'weight_reps', false, ARRAY['back','biceps']),
    ('Dumbbell row',           'weight_reps', false, ARRAY['back','biceps']),
    ('Lat pulldown',           'weight_reps', false, ARRAY['back','biceps']),
    ('Seated cable row',       'weight_reps', false, ARRAY['back','biceps']),
    ('Hip thrust',             'weight_reps', false, ARRAY['glutes','hamstrings']),
    ('Power clean',            'weight_reps', false, ARRAY['full_body']),
    ('Leg press',              'weight_reps', false, ARRAY['quads','glutes']),
    ('Leg curl',               'weight_reps', false, ARRAY['hamstrings']),
    ('Leg extension',          'weight_reps', false, ARRAY['quads']),
    ('Calf raise',             'weight_reps', false, ARRAY['calves']),
    ('Bulgarian split squat',  'weight_reps', false, ARRAY['quads','glutes']),
    ('Lunge',                  'weight_reps', false, ARRAY['quads','glutes']),
    ('Chest fly',              'weight_reps', false, ARRAY['chest']),
    ('Lateral raise',          'weight_reps', false, ARRAY['shoulders']),
    ('Face pull',              'weight_reps', false, ARRAY['shoulders','back']),
    ('Bicep curl',             'weight_reps', false, ARRAY['biceps']),
    ('Hammer curl',            'weight_reps', false, ARRAY['biceps','forearms']),
    ('Tricep pushdown',        'weight_reps', false, ARRAY['triceps']),
    ('Skull crusher',          'weight_reps', false, ARRAY['triceps']),
    ('Kettlebell swing',       'weight_reps', false, ARRAY['glutes','hamstrings']),
    ('Wall ball',              'weight_reps', false, ARRAY['full_body']),
    -- reps only
    ('Pull-up',                'reps', false, ARRAY['back','biceps']),
    ('Chin-up',                'reps', false, ARRAY['back','biceps']),
    ('Push-up',                'reps', false, ARRAY['chest','triceps','shoulders']),
    ('Dip',                    'reps', false, ARRAY['chest','triceps']),
    ('Sit-up',                 'reps', false, ARRAY['core']),
    ('Toes to bar',            'reps', false, ARRAY['core']),
    ('Burpee',                 'reps', false, ARRAY['full_body']),
    ('Air squat',              'reps', false, ARRAY['quads','glutes']),
    ('Box jump',               'reps', false, ARRAY['quads','glutes']),
    ('Walking lunge',          'reps', false, ARRAY['quads','glutes']),
    ('Mountain climber',       'reps', false, ARRAY['core','cardio']),
    ('Handstand push-up',      'reps', false, ARRAY['shoulders','triceps']),
    -- distance (loaded = a weight is optional)
    ('SkiErg',                 'distance', false, ARRAY['full_body','cardio']),
    ('Rowing machine',         'distance', false, ARRAY['full_body','cardio']),
    ('Run',                    'distance', false, ARRAY['cardio']),
    ('Assault bike',           'distance', false, ARRAY['cardio']),
    ('Burpee broad jump',      'distance', false, ARRAY['full_body']),
    ('Sled push',              'distance', true,  ARRAY['quads','glutes','calves']),
    ('Sled pull',              'distance', true,  ARRAY['hamstrings','back']),
    ('Farmers carry',          'distance', true,  ARRAY['forearms','core','shoulders']),
    ('Sandbag lunges',         'distance', true,  ARRAY['quads','glutes']),
    -- time
    ('Plank',                  'time', false, ARRAY['core']),
    ('Hollow hold',            'time', false, ARRAY['core']),
    ('Wall sit',               'time', false, ARRAY['quads']),
    ('Dead hang',              'time', false, ARRAY['forearms','back']),
    ('Battle ropes',           'time', false, ARRAY['shoulders','cardio']),
    ('Jump rope',              'time', false, ARRAY['cardio','calves']),
    ('Weighted carry hold',    'time', true,  ARRAY['forearms','core'])
) AS s(name, measure, loaded, muscles)
ON CONFLICT (key) DO NOTHING;

-- Names that only exist in activity_lifts (free text before the library) become library rows
-- with no muscles; an admin can tag or merge them.
INSERT INTO exercises (id, name, key, measure, loaded, muscles, created_by, created_at)
SELECT gen_random_uuid()::TEXT, l.exercise, l.exercise, 'weight_reps', false, '{}', NULL, extract(epoch FROM now())::BIGINT
FROM (SELECT DISTINCT exercise FROM activity_lifts) l
ON CONFLICT (key) DO NOTHING;

-- One 1-round block with one station per legacy lift row, skipping activities that already
-- have blocks (so a second run, or a row the service already wrote, is left alone).
WITH todo AS (
    SELECT l.id AS lift_id, l.activity_id,
           row_number() OVER (PARTITION BY l.activity_id ORDER BY l.id) - 1 AS pos
    FROM activity_lifts l
    WHERE NOT EXISTS (SELECT 1 FROM activity_blocks b WHERE b.activity_id = l.activity_id)
), new_blocks AS (
    INSERT INTO activity_blocks (activity_id, position, rounds)
    SELECT activity_id, pos, 1 FROM todo
    RETURNING id, activity_id, position
)
INSERT INTO activity_stations (block_id, position, exercise_id, weight_kg, reps)
SELECT nb.id, 0, e.id, l.weight_kg, l.reps
FROM new_blocks nb
JOIN todo t ON t.activity_id = nb.activity_id AND t.pos = nb.position
JOIN activity_lifts l ON l.id = t.lift_id
JOIN exercises e ON e.key = l.exercise;

-- +goose Down
-- Data only: 00008's Down drops the tables these rows live in.
SELECT 1;
