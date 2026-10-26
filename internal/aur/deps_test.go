package aur

import (
	"errors"
	"testing"
)

// fakeAUR serves PKGBUILDs from a map so the graph logic runs without network.
func fakeAUR(pkgbuilds map[string]string) PackageFetcher {
	return func(name string) (string, error) {
		content, ok := pkgbuilds[name]
		if !ok {
			return "", errors.New("not in the AUR")
		}
		return content, nil
	}
}

func indexOf(names []string, want string) int {
	for i, n := range names {
		if n == want {
			return i
		}
	}
	return -1
}

func TestResolveBuildOrder_DependenciesComeFirst(t *testing.T) {
	fetch := fakeAUR(map[string]string{
		"app": `pkgname=app
depends=('middle' 'glibc')
`,
		"middle": `pkgname=middle
depends=('base-lib')
`,
		"base-lib": `pkgname=base-lib
depends=('glibc')
`,
	})

	order, warnings := resolveBuildOrder(
		[]string{"app"}, fetch,
		func(name string) bool { return name == "glibc" },
		4,
	)

	want := []string{"base-lib", "middle", "app"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v (warnings: %v)", order, want, warnings)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestResolveBuildOrder_OfficialDependenciesExcluded(t *testing.T) {
	fetch := fakeAUR(map[string]string{
		"app": `pkgname=app
depends=('glibc' 'gcc-libs')
makedepends=('go')
`,
	})

	order, warnings := resolveBuildOrder(
		[]string{"app"}, fetch,
		func(name string) bool {
			return name == "glibc" || name == "gcc-libs" || name == "go"
		},
		2,
	)

	if len(order) != 1 || order[0] != "app" {
		t.Fatalf("order = %v, want [app]; warnings: %v", order, warnings)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestResolveBuildOrder_MakeAndCheckDependsCount(t *testing.T) {
	fetch := fakeAUR(map[string]string{
		"app": `pkgname=app
makedepends=('builder')
checkdepends=('checker')
`,
		"builder": `pkgname=builder`,
		"checker": `pkgname=checker`,
	})

	order, _ := resolveBuildOrder([]string{"app"}, fetch, func(string) bool { return false }, 4)

	for _, dep := range []string{"builder", "checker"} {
		if indexOf(order, dep) < 0 {
			t.Fatalf("expected %s in order, got %v", dep, order)
		}
	}
	if indexOf(order, "app") < indexOf(order, "builder") {
		t.Errorf("app must come after builder, got %v", order)
	}
	if indexOf(order, "app") < indexOf(order, "checker") {
		t.Errorf("app must come after checker, got %v", order)
	}
}

func TestResolveBuildOrder_VersionedDependencyIsStripped(t *testing.T) {
	fetch := fakeAUR(map[string]string{
		"app": `pkgname=app
depends=('libfoo>=2.0')
`,
		"libfoo": `pkgname=libfoo`,
	})

	order, warnings := resolveBuildOrder([]string{"app"}, fetch, func(string) bool { return false }, 2)

	if indexOf(order, "libfoo") < 0 {
		t.Fatalf("expected libfoo>=2.0 to resolve to libfoo, got order %v (warnings %v)", order, warnings)
	}
}

func TestResolveBuildOrder_CycleDoesNotHang(t *testing.T) {
	fetch := fakeAUR(map[string]string{
		"a": `pkgname=a
depends=('b')
`,
		"b": `pkgname=b
depends=('a')
`,
	})

	order, warnings := resolveBuildOrder([]string{"a"}, fetch, func(string) bool { return false }, 2)

	if len(order) != 2 {
		t.Fatalf("expected both packages in order, got %v", order)
	}
	if len(warnings) == 0 {
		t.Error("expected a cycle warning")
	}
}

func TestResolveBuildOrder_MissingRequestedPackageStillReturned(t *testing.T) {
	order, warnings := resolveBuildOrder(
		[]string{"absent"}, fakeAUR(nil), func(string) bool { return false }, 2,
	)

	if len(order) != 1 || order[0] != "absent" {
		t.Fatalf("order = %v, want [absent]", order)
	}
	if len(warnings) == 0 {
		t.Error("expected a warning for the unreadable PKGBUILD")
	}
}

func TestResolveBuildOrder_UnresolvableDependencyIsPruned(t *testing.T) {
	// A dependency that is neither in a repository nor in the AUR is usually a
	// virtual provide. It must not appear in the build order.
	fetch := fakeAUR(map[string]string{
		"app": `pkgname=app
depends=('virtual-thing' 'realdep')
`,
		"realdep": `pkgname=realdep`,
	})

	order, warnings := resolveBuildOrder([]string{"app"}, fetch, func(string) bool { return false }, 2)

	if indexOf(order, "virtual-thing") >= 0 {
		t.Fatalf("unresolvable dependency must be pruned, got order %v", order)
	}
	if indexOf(order, "realdep") < 0 || indexOf(order, "app") < 0 {
		t.Fatalf("expected realdep and app in order, got %v", order)
	}
	if len(warnings) == 0 {
		t.Error("expected a note about the skipped dependency")
	}
}

func TestResolveBuildOrder_DeduplicatesRequests(t *testing.T) {
	fetch := fakeAUR(map[string]string{"app": "pkgname=app\n"})

	order, _ := resolveBuildOrder([]string{"app", "app", "app"}, fetch, func(string) bool { return false }, 2)

	if len(order) != 1 {
		t.Fatalf("order = %v, want one entry", order)
	}
}

func TestResolveBuildOrder_MultipleRoots(t *testing.T) {
	fetch := fakeAUR(map[string]string{
		"one": `pkgname=one`,
		"two": `pkgname=two
depends=('one')
`,
	})

	order, _ := resolveBuildOrder([]string{"two", "one"}, fetch, func(string) bool { return false }, 2)

	if len(order) != 2 {
		t.Fatalf("order = %v, want two entries", order)
	}
	if indexOf(order, "one") > indexOf(order, "two") {
		t.Errorf("one must be built before two, got %v", order)
	}
}

func TestResolveBuildOrder_EmptyInput(t *testing.T) {
	order, warnings := ResolveBuildOrder(nil, nil, 4)
	if order != nil || warnings != nil {
		t.Errorf("expected empty results, got %v / %v", order, warnings)
	}
}
