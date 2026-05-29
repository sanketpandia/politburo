package vaadmin

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/platform/ui"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

func TestMain(m *testing.M) {
	_ = logging.Init("local")
	os.Exit(m.Run())
}

func testVaadminRenderer() *templates.Renderer {
	return templates.NewRenderer(
		"templates",
		"templates/partials",
		"templates/layouts/base.html",
	)
}

func TestBuildDatasourceStatusCardView(t *testing.T) {
	view := buildDatasourceStatusCardView(map[string]*platformVA.SchemaConfig{
		"pilot": &platformVA.SchemaConfig{Enabled: true},
		"route": &platformVA.SchemaConfig{Enabled: true},
	})

	if view.Total != 4 {
		t.Fatalf("Total = %d, want 4", view.Total)
	}
	if view.Configured != 2 {
		t.Fatalf("Configured = %d, want 2", view.Configured)
	}
	if view.AllConfigured {
		t.Fatalf("AllConfigured = true, want false")
	}
	if len(view.Rows) != 4 {
		t.Fatalf("Rows len = %d, want 4", len(view.Rows))
	}
	if !view.Rows[0].Configured || !view.Rows[1].Configured {
		t.Fatalf("expected first two rows to be configured")
	}
	if view.Rows[2].Configured || view.Rows[3].Configured {
		t.Fatalf("expected last two rows to be missing")
	}
}

func TestDatasourceStatusPartialRendersConfiguredChecks(t *testing.T) {
	renderer := testVaadminRenderer()
	view := buildDatasourceStatusCardView(map[string]*platformVA.SchemaConfig{
		"pilot": &platformVA.SchemaConfig{Enabled: true},
	})

	rr := httptest.NewRecorder()
	data := map[string]interface{}{
		"ActiveVA": session.VAMembership{VAName: "Test Virtual"},
		"Status":   view,
	}

	if err := renderer.RenderPartial(rr, "partials/vaadmin-datasource-status.html", data); err != nil {
		t.Fatalf("RenderPartial() error = %v", err)
	}

	body := rr.Body.String()
	for _, want := range []string{
		"Datasource status",
		"4 schema types",
		"Pilot",
		"✓",
		"1/4",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected rendered body to contain %q, got:\n%s", want, body)
		}
	}
}

func TestVaadminIndexPageIncludesDatasourceStatusCard(t *testing.T) {
	renderer := testVaadminRenderer()

	rr := httptest.NewRecorder()
	data := map[string]interface{}{
		"PageTitle":   "VA Admin",
		"ActiveVA":    session.VAMembership{VAName: "Test Virtual", Role: "admin"},
		"CurrentPage": "vaadmin-pilots",
		"MenuItems":   ui.GetMenuItems(&session.VAMembership{VAName: "Test Virtual", Role: "admin"}),
	}

	if err := renderer.RenderTemplate(rr, "pages/vaadmin-index.html", data); err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}

	body := rr.Body.String()
	for _, want := range []string{
		"Setup datasource",
		`hx-get="/dashboard/vaadmin/datasource/status"`,
		`href="/dashboard/settings/datasource"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected rendered body to contain %q, got:\n%s", want, body)
		}
	}
	if strings.Contains(body, "secondary-nav") {
		t.Fatalf("expected vaadmin index page to omit secondary-nav, got:\n%s", body)
	}
}
