package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	fitterv1 "github.com/fitter/svc/gen/fitter/v1"
	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/service"
)

// adminFixture seeds an active admin (role in the DB and in the context) and returns the service.
func adminFixture(t *testing.T) (*service.Deps, *service.AdminService, db.User) {
	t.Helper()
	deps := newDeps(t)
	admin := seedUser(t, deps.Queries, "admin")
	ctx := context.Background()
	if err := deps.Queries.SetUserStatus(ctx, db.SetUserStatusParams{ID: admin.ID, Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := deps.Queries.SetUserRole(ctx, db.SetUserRoleParams{ID: admin.ID, Role: "admin"}); err != nil {
		t.Fatal(err)
	}
	return deps, service.NewAdminService(deps), admin
}

func setStatus(t *testing.T, deps *service.Deps, id, status string) {
	t.Helper()
	if err := deps.Queries.SetUserStatus(context.Background(), db.SetUserStatusParams{ID: id, Status: status}); err != nil {
		t.Fatal(err)
	}
}

func totalAudit(t *testing.T, deps *service.Deps) int {
	t.Helper()
	var n int
	if err := deps.DB.QueryRow(`SELECT count(*) FROM admin_actions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Every admin RPC refuses a plain member, and none of them touches the audit log when it does.
func TestEveryAdminRPCRefusesMembers(t *testing.T) {
	deps := newDeps(t)
	alice := seedUser(t, deps.Queries, "alice")
	svc := service.NewAdminService(deps)
	ctx := ctxAs(alice.ID)
	id := "x"
	calls := map[string]func() error{
		"ListUsers": func() error { _, e := svc.ListUsers(ctx, connect.NewRequest(&fitterv1.ListUsersRequest{})); return e },
		"ApproveUser": func() error {
			_, e := svc.ApproveUser(ctx, connect.NewRequest(&fitterv1.ApproveUserRequest{Id: id}))
			return e
		},
		"BanUser": func() error {
			_, e := svc.BanUser(ctx, connect.NewRequest(&fitterv1.BanUserRequest{Id: id, Reason: "r"}))
			return e
		},
		"UnbanUser": func() error {
			_, e := svc.UnbanUser(ctx, connect.NewRequest(&fitterv1.UnbanUserRequest{Id: id}))
			return e
		},
		"SetUserRole": func() error {
			_, e := svc.SetUserRole(ctx, connect.NewRequest(&fitterv1.SetUserRoleRequest{Id: id, Role: "admin"}))
			return e
		},
		"ListAdminActions": func() error {
			_, e := svc.ListAdminActions(ctx, connect.NewRequest(&fitterv1.ListAdminActionsRequest{}))
			return e
		},
		"UpdateExercise": func() error {
			_, e := svc.UpdateExercise(ctx, connect.NewRequest(&fitterv1.UpdateExerciseRequest{Id: id}))
			return e
		},
		"DeleteExercise": func() error {
			_, e := svc.DeleteExercise(ctx, connect.NewRequest(&fitterv1.DeleteExerciseRequest{Id: id}))
			return e
		},
		"MergeExercises": func() error {
			_, e := svc.MergeExercises(ctx, connect.NewRequest(&fitterv1.MergeExercisesRequest{SourceId: "a", TargetId: "b"}))
			return e
		},
		"SetExerciseArchived": func() error {
			_, e := svc.SetExerciseArchived(ctx, connect.NewRequest(&fitterv1.SetExerciseArchivedRequest{Id: id, Archived: true}))
			return e
		},
	}
	for name, call := range calls {
		if err := call(); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s as a member: got %v, want PermissionDenied", name, err)
		}
	}
	if n := totalAudit(t, deps); n != 0 {
		t.Fatalf("%d audit rows written by refused calls", n)
	}
}

func TestApproveUser_OnlyPendingUsersAndMissingIDs(t *testing.T) {
	deps, svc, admin := adminFixture(t)
	as := ctxAsAdmin(admin.ID)
	active, banned, pending := seedUser(t, deps.Queries, "active"), seedUser(t, deps.Queries, "banned"), seedUser(t, deps.Queries, "pending")
	setStatus(t, deps, active.ID, "active")
	setStatus(t, deps, banned.ID, "banned")

	for name, id := range map[string]string{"already active": active.ID, "banned": banned.ID} {
		if _, err := svc.ApproveUser(as, connect.NewRequest(&fitterv1.ApproveUserRequest{Id: id})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("approve %s: got %v, want FailedPrecondition", name, err)
		}
	}
	for _, id := range []string{"", "ghost"} {
		if _, err := svc.ApproveUser(as, connect.NewRequest(&fitterv1.ApproveUserRequest{Id: id})); connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("approve id %q: got %v, want NotFound", id, err)
		}
	}
	if n := totalAudit(t, deps); n != 0 {
		t.Fatalf("%d audit rows after only refused approvals", n)
	}
	if _, err := svc.ApproveUser(as, connect.NewRequest(&fitterv1.ApproveUserRequest{Id: pending.ID})); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveUser(as, connect.NewRequest(&fitterv1.ApproveUserRequest{Id: pending.ID})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("approving twice: got %v, want FailedPrecondition", err)
	}
	if n := totalAudit(t, deps); n != 1 {
		t.Fatalf("audit rows = %d, want exactly the one approval", n)
	}
}

func TestBanUser_ReasonRulesAndStates(t *testing.T) {
	deps, svc, admin := adminFixture(t)
	as := ctxAsAdmin(admin.ID)
	ban := func(id, reason string) error {
		_, err := svc.BanUser(as, connect.NewRequest(&fitterv1.BanUserRequest{Id: id, Reason: reason}))
		return err
	}
	u := seedUser(t, deps.Queries, "u")

	// A reason that is only whitespace explains nothing, so it counts as missing.
	for _, r := range []string{"", " ", "   \t\n"} {
		if err := ban(u.ID, r); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("reason %q: got %v, want InvalidArgument", r, err)
		}
	}
	// The cap counts characters, not bytes: 500 accented letters are fine, 501 are not.
	if err := ban(u.ID, strings.Repeat("é", 501)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("501 characters: got %v, want InvalidArgument", err)
	}
	if err := ban("ghost", "x"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("ban unknown user: got %v, want NotFound", err)
	}
	if n := totalAudit(t, deps); n != 0 {
		t.Fatalf("%d audit rows after only refused bans", n)
	}

	if err := ban(u.ID, strings.Repeat("é", 500)); err != nil {
		t.Fatalf("500 accented characters: %v", err)
	}
	if err := ban(u.ID, "again"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("banning a banned user: got %v, want FailedPrecondition", err)
	}
	got, _ := deps.Queries.GetUserByID(context.Background(), u.ID)
	if got.BanReason != strings.Repeat("é", 500) {
		t.Errorf("the refused second ban overwrote the stored reason")
	}
}

func TestUnbanUser_OnlyBannedUsers(t *testing.T) {
	deps, svc, admin := adminFixture(t)
	as := ctxAsAdmin(admin.ID)
	active := seedUser(t, deps.Queries, "active")
	setStatus(t, deps, active.ID, "active")
	pending := seedUser(t, deps.Queries, "pending")
	for _, id := range []string{active.ID, pending.ID} {
		if _, err := svc.UnbanUser(as, connect.NewRequest(&fitterv1.UnbanUserRequest{Id: id})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("unban a user who isn't banned: got %v, want FailedPrecondition", err)
		}
	}
	if _, err := svc.UnbanUser(as, connect.NewRequest(&fitterv1.UnbanUserRequest{Id: "ghost"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unban unknown user: got %v, want NotFound", err)
	}
	if n := totalAudit(t, deps); n != 0 {
		t.Fatalf("%d audit rows after only refused unbans", n)
	}
}

func TestSetUserRole_EdgeCases(t *testing.T) {
	deps, svc, admin := adminFixture(t)
	as := ctxAsAdmin(admin.ID)
	set := func(id, role string) error {
		_, err := svc.SetUserRole(as, connect.NewRequest(&fitterv1.SetUserRoleRequest{Id: id, Role: role}))
		return err
	}
	active, pending, banned := seedUser(t, deps.Queries, "active"), seedUser(t, deps.Queries, "pending"), seedUser(t, deps.Queries, "banned")
	setStatus(t, deps, active.ID, "active")
	setStatus(t, deps, banned.ID, "banned")

	for _, role := range []string{"", "owner", "ADMIN", "Admin", " member", "member "} {
		if err := set(active.ID, role); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("role %q: got %v, want InvalidArgument", role, err)
		}
	}
	if err := set("ghost", "admin"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown user: got %v, want NotFound", err)
	}
	for name, id := range map[string]string{"pending": pending.ID, "banned": banned.ID} {
		if err := set(id, "admin"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("promote a %s user: got %v, want FailedPrecondition", name, err)
		}
	}
	if err := set(active.ID, "member"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("demote a member: got %v, want FailedPrecondition (already that role)", err)
	}
	if err := set(admin.ID, "admin"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("promote an admin: got %v, want FailedPrecondition", err)
	}
	if n := totalAudit(t, deps); n != 0 {
		t.Fatalf("%d audit rows after only refused role changes", n)
	}

	// With two active admins either can be demoted by the other, once.
	if err := set(active.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	if err := set(active.ID, "member"); err != nil {
		t.Fatalf("demote with two admins: %v", err)
	}
	// A banned admin doesn't count as an active admin: demoting them is allowed even when
	// they're the only other one, and the last *active* admin stays protected.
	if _, err := deps.DB.Exec(`UPDATE users SET role='admin' WHERE id=$1`, banned.ID); err != nil {
		t.Fatal(err)
	}
	if err := set(banned.ID, "member"); err != nil {
		t.Fatalf("demote a banned admin: %v", err)
	}
}

func TestListUsers_StatusFilter(t *testing.T) {
	deps, svc, admin := adminFixture(t)
	as := ctxAsAdmin(admin.ID)
	a, b := seedUser(t, deps.Queries, "b-banned"), seedUser(t, deps.Queries, "c-pending")
	setStatus(t, deps, a.ID, "banned")
	_ = b
	list := func(status string) []string {
		r, err := svc.ListUsers(as, connect.NewRequest(&fitterv1.ListUsersRequest{Status: status}))
		if err != nil {
			t.Fatalf("ListUsers(%q): %v", status, err)
		}
		var names []string
		for _, u := range r.Msg.GetUsers() {
			names = append(names, u.GetName())
		}
		return names
	}
	if got := list(""); len(got) != 3 {
		t.Errorf("all users = %v, want 3", got)
	}
	for status, want := range map[string]string{"active": "admin", "banned": "b-banned", "pending": "c-pending"} {
		if got := list(status); len(got) != 1 || got[0] != want {
			t.Errorf("status %q = %v, want [%s]", status, got, want)
		}
	}
	if got := list("nonsense"); len(got) != 0 {
		t.Errorf("unknown status = %v, want none", got)
	}
}

func TestListAdminActions_LimitAndOrder(t *testing.T) {
	deps, svc, admin := adminFixture(t)
	as := ctxAsAdmin(admin.ID)
	for i, ts := range []int64{100, 300, 200} {
		if _, err := deps.DB.Exec(`INSERT INTO admin_actions (id, actor_id, action, target_id, detail, created_at) VALUES ($1,$2,'approve','t','d',$3)`, string(rune('a'+i)), admin.ID, ts); err != nil {
			t.Fatal(err)
		}
	}
	list := func(limit int32) []int64 {
		r, err := svc.ListAdminActions(as, connect.NewRequest(&fitterv1.ListAdminActionsRequest{Limit: limit}))
		if err != nil {
			t.Fatal(err)
		}
		var out []int64
		for _, a := range r.Msg.GetActions() {
			out = append(out, a.GetCreatedAt().AsTime().Unix())
		}
		return out
	}
	if got := list(0); len(got) != 3 || got[0] != 300 || got[1] != 200 || got[2] != 100 {
		t.Errorf("default limit = %v, want all three newest first", got)
	}
	if got := list(-5); len(got) != 3 {
		t.Errorf("negative limit = %v, want the default (all three)", got)
	}
	if got := list(2); len(got) != 2 || got[0] != 300 {
		t.Errorf("limit 2 = %v, want the two newest", got)
	}
	if got := list(1_000_000); len(got) != 3 {
		t.Errorf("huge limit = %v, want it capped but still returning everything here", got)
	}
}

// --- Exercises -------------------------------------------------------------------------

func logWeighted(t *testing.T, deps *service.Deps, userID, key string) {
	t.Helper()
	_, err := service.NewActivityService(deps).CreateManualActivity(ctxAs(userID), connect.NewRequest(&fitterv1.CreateManualActivityRequest{
		Activity: circuit(&fitterv1.Block{Rounds: 1, Stations: []*fitterv1.Station{stationOf(deps, t, key, func(s *fitterv1.Station) { s.WeightKg, s.Reps = f64(60), i32(5) })}}),
	}))
	if err != nil {
		t.Fatalf("log %s: %v", key, err)
	}
}

func TestUpdateExercise_FieldValidationAndRenames(t *testing.T) {
	deps, svc, admin := adminFixture(t)
	as := ctxAsAdmin(admin.ID)
	id := exerciseID(t, deps, "plank") // time, unloaded, unused
	upd := func(name string, m fitterv1.ExerciseMeasure, loaded bool, muscles ...fitterv1.Muscle) (*fitterv1.Exercise, error) {
		r, err := svc.UpdateExercise(as, connect.NewRequest(&fitterv1.UpdateExerciseRequest{Id: id, Name: name, Measure: m, Loaded: loaded, Muscles: muscles}))
		if err != nil {
			return nil, err
		}
		return r.Msg.GetExercise(), nil
	}
	T, core := fitterv1.ExerciseMeasure_EXERCISE_MEASURE_TIME, fitterv1.Muscle_MUSCLE_CORE

	for name, call := range map[string]func() error{
		"empty name":      func() error { _, e := upd("", T, false, core); return e },
		"whitespace name": func() error { _, e := upd("   ", T, false, core); return e },
		"61 characters":   func() error { _, e := upd(strings.Repeat("a", 61), T, false, core); return e },
		"no muscles":      func() error { _, e := upd("Plank", T, false); return e },
		"unknown muscle":  func() error { _, e := upd("Plank", T, false, fitterv1.Muscle(99)); return e },
		"unspecified measure": func() error {
			_, e := upd("Plank", fitterv1.ExerciseMeasure_EXERCISE_MEASURE_UNSPECIFIED, false, core)
			return e
		},
		"loaded on reps": func() error {
			_, e := upd("Plank", fitterv1.ExerciseMeasure_EXERCISE_MEASURE_REPS, true, core)
			return e
		},
		"loaded on weight×reps": func() error {
			_, e := upd("Plank", fitterv1.ExerciseMeasure_EXERCISE_MEASURE_WEIGHT_REPS, true, core)
			return e
		},
	} {
		if err := call(); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}
	if _, err := svc.UpdateExercise(as, connect.NewRequest(&fitterv1.UpdateExerciseRequest{Id: "ghost", Name: "x", Measure: T, Muscles: []fitterv1.Muscle{core}})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown id: got %v, want NotFound", err)
	}
	if n := auditCount(t, deps, "update_exercise"); n != 0 {
		t.Fatalf("%d audit rows after only refused updates", n)
	}

	// A case-only rename keeps the key, so it isn't a collision with itself. Spacing is folded.
	e, err := upd("  PLANK   hold ", T, false, core, core, fitterv1.Muscle_MUSCLE_BACK)
	if err != nil || e.GetName() != "PLANK hold" || e.GetKey() != "plank hold" || len(e.GetMuscles()) != 2 {
		t.Fatalf("rename: %+v err=%v, want name folded, key lower-cased, duplicate muscles removed", e, err)
	}
	if e, err = upd("Plank hold", T, false, core); err != nil || e.GetKey() != "plank hold" {
		t.Fatalf("case-only rename: %+v err=%v", e, err)
	}
	// Unused exercises may change how they're measured, including gaining the optional weight.
	if e, err = upd("Plank hold", T, true, core); err != nil || !e.GetLoaded() {
		t.Fatalf("make loaded while unused: %+v err=%v", e, err)
	}
	if e, err = upd("Plank hold", fitterv1.ExerciseMeasure_EXERCISE_MEASURE_REPS, false, core); err != nil || e.GetMeasure() != fitterv1.ExerciseMeasure_EXERCISE_MEASURE_REPS {
		t.Fatalf("change measure while unused: %+v err=%v", e, err)
	}
}

func TestUpdateExercise_UsedExerciseKeepsItsMeasureAndLoadedFlag(t *testing.T) {
	deps, svc, admin := adminFixture(t)
	alice := seedUser(t, deps.Queries, "alice")
	logWeighted(t, deps, alice.ID, "squat")
	id := exerciseID(t, deps, "squat")
	as := ctxAsAdmin(admin.ID)
	W, q := fitterv1.ExerciseMeasure_EXERCISE_MEASURE_WEIGHT_REPS, []fitterv1.Muscle{fitterv1.Muscle_MUSCLE_QUADS}
	for name, req := range map[string]*fitterv1.UpdateExerciseRequest{
		"to reps": {Id: id, Name: "Squat", Measure: fitterv1.ExerciseMeasure_EXERCISE_MEASURE_REPS, Muscles: q},
		"to time": {Id: id, Name: "Squat", Measure: fitterv1.ExerciseMeasure_EXERCISE_MEASURE_TIME, Muscles: q},
	} {
		if _, err := svc.UpdateExercise(as, connect.NewRequest(req)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s while used: got %v, want InvalidArgument", name, err)
		}
	}
	// Name and muscles are still editable.
	if r, err := svc.UpdateExercise(as, connect.NewRequest(&fitterv1.UpdateExerciseRequest{Id: id, Name: "Squat", Measure: W, Muscles: append(q, fitterv1.Muscle_MUSCLE_CORE)})); err != nil || len(r.Msg.GetExercise().GetMuscles()) != 2 {
		t.Fatalf("edit muscles while used: %v %v", r, err)
	}
}

func TestArchive_IsIdempotentAndAudited(t *testing.T) {
	deps, svc, admin := adminFixture(t)
	as := ctxAsAdmin(admin.ID)
	id := exerciseID(t, deps, "plank")
	set := func(on bool) (*fitterv1.Exercise, error) {
		r, err := svc.SetExerciseArchived(as, connect.NewRequest(&fitterv1.SetExerciseArchivedRequest{Id: id, Archived: on}))
		if err != nil {
			return nil, err
		}
		return r.Msg.GetExercise(), nil
	}
	if _, err := svc.SetExerciseArchived(as, connect.NewRequest(&fitterv1.SetExerciseArchivedRequest{Id: "ghost", Archived: true})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("archive unknown: got %v, want NotFound", err)
	}
	if e, err := set(false); err != nil || e.GetArchived() { // restoring a live exercise is a no-op
		t.Fatalf("restore a live exercise: %+v %v", e, err)
	}
	if n := auditCount(t, deps, "restore_exercise"); n != 0 {
		t.Fatalf("no-op restore wrote %d audit rows", n)
	}
	if e, err := set(true); err != nil || !e.GetArchived() {
		t.Fatalf("archive: %+v %v", e, err)
	}
	var first int64
	_ = deps.DB.QueryRow(`SELECT archived_at FROM exercises WHERE id=$1`, id).Scan(&first)
	time.Sleep(1100 * time.Millisecond)
	if _, err := set(true); err != nil {
		t.Fatal(err)
	}
	var second int64
	_ = deps.DB.QueryRow(`SELECT archived_at FROM exercises WHERE id=$1`, id).Scan(&second)
	if first != second {
		t.Errorf("re-archiving moved archived_at from %d to %d", first, second)
	}
	if n := auditCount(t, deps, "archive_exercise"); n != 1 {
		t.Errorf("archive audit rows = %d, want 1 for two archive calls", n)
	}
	if e, err := set(false); err != nil || e.GetArchived() {
		t.Fatalf("restore: %+v %v", e, err)
	}
	if n := auditCount(t, deps, "restore_exercise"); n != 1 {
		t.Errorf("restore audit rows = %d, want 1", n)
	}
}

func TestMergeExercises_Refusals(t *testing.T) {
	deps, svc, admin := adminFixture(t)
	alice := seedUser(t, deps.Queries, "alice")
	as := ctxAsAdmin(admin.ID)
	merge := func(src, dst string) error {
		_, err := svc.MergeExercises(as, connect.NewRequest(&fitterv1.MergeExercisesRequest{SourceId: src, TargetId: dst}))
		return err
	}
	squat, front, plank, sled, ski := exerciseID(t, deps, "squat"), exerciseID(t, deps, "front squat"), exerciseID(t, deps, "plank"), exerciseID(t, deps, "sled push"), exerciseID(t, deps, "skierg")

	if err := merge(squat, squat); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("merge into itself: got %v, want InvalidArgument", err)
	}
	if err := merge("ghost", front); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown source: got %v, want NotFound", err)
	}
	if err := merge(squat, "ghost"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown target: got %v, want NotFound", err)
	}
	if err := merge(squat, plank); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("different measures: got %v, want InvalidArgument", err)
	}
	// A loaded source can't fold into an unloaded target (its weights would have nowhere to go).
	if err := merge(sled, ski); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("loaded into unloaded: got %v, want InvalidArgument", err)
	}
	// An archived target would hide the merged history from new logs.
	if _, err := svc.SetExerciseArchived(as, connect.NewRequest(&fitterv1.SetExerciseArchivedRequest{Id: front, Archived: true})); err != nil {
		t.Fatal(err)
	}
	if err := merge(squat, front); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("into an archived target: got %v, want InvalidArgument", err)
	}
	if n := auditCount(t, deps, "merge_exercise"); n != 0 {
		t.Fatalf("%d audit rows after only refused merges", n)
	}
	// Every refusal left both exercises in place.
	for _, id := range []string{squat, front, plank, sled, ski} {
		if _, err := deps.Queries.GetExercise(context.Background(), id); err != nil {
			t.Fatalf("exercise %s vanished after a refused merge: %v", id, err)
		}
	}

	// An unloaded source may fold into a loaded target, and an archived source may be merged away.
	logWeighted(t, deps, alice.ID, "squat")
	if _, err := svc.SetExerciseArchived(as, connect.NewRequest(&fitterv1.SetExerciseArchivedRequest{Id: squat, Archived: true})); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetExerciseArchived(as, connect.NewRequest(&fitterv1.SetExerciseArchivedRequest{Id: front, Archived: false})); err != nil {
		t.Fatal(err)
	}
	if err := merge(squat, front); err != nil {
		t.Fatalf("merge an archived, used source into a live target: %v", err)
	}
	var stations int
	_ = deps.DB.QueryRow(`SELECT count(*) FROM activity_stations WHERE exercise_id=$1`, front).Scan(&stations)
	if stations != 1 {
		t.Fatalf("target has %d stations after the merge, want the 1 moved over", stations)
	}
}

func TestDeleteExercise_EdgeCases(t *testing.T) {
	deps, svc, admin := adminFixture(t)
	as := ctxAsAdmin(admin.ID)
	del := func(id string) error {
		_, err := svc.DeleteExercise(as, connect.NewRequest(&fitterv1.DeleteExerciseRequest{Id: id}))
		return err
	}
	if err := del(""); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("empty id: got %v, want NotFound", err)
	}
	// An archived but unused exercise can still be deleted outright.
	id := exerciseID(t, deps, "plank")
	if _, err := svc.SetExerciseArchived(as, connect.NewRequest(&fitterv1.SetExerciseArchivedRequest{Id: id, Archived: true})); err != nil {
		t.Fatal(err)
	}
	if err := del(id); err != nil {
		t.Fatalf("delete archived unused: %v", err)
	}
	// A goal that tracks an exercise counts as use even when nothing was logged.
	alice := seedUser(t, deps.Queries, "alice")
	key := "deadlift"
	if _, err := service.NewGoalService(deps).CreateGoal(ctxAs(alice.ID), connect.NewRequest(&fitterv1.CreateGoalRequest{
		Name: "Pull 180", PeriodStart: ts(1), PeriodEnd: ts(2),
		Clauses: []*fitterv1.GoalClause{{
			Metric: fitterv1.ActivityMetric_ACTIVITY_METRIC_MAX_WEIGHT_KG, Scope: fitterv1.ClauseScope_CLAUSE_SCOPE_AGGREGATE,
			Comparator: fitterv1.Comparator_COMPARATOR_GTE, Value: 180, Exercise: &key,
		}},
	})); err != nil {
		t.Fatal(err)
	}
	if err := del(exerciseID(t, deps, "deadlift")); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("delete an exercise a goal tracks: got %v, want FailedPrecondition", err)
	}
}
