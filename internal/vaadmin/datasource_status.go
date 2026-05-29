package vaadmin

import (
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

type datasourceSchemaStatusRow struct {
	Label      string
	Configured bool
}

type datasourceStatusCardView struct {
	Rows          []datasourceSchemaStatusRow
	Configured    int
	Total         int
	AllConfigured bool
}

func buildDatasourceStatusCardView(schemas map[string]*platformVA.SchemaConfig) datasourceStatusCardView {
	rows := []datasourceSchemaStatusRow{
		{
			Label:      "Pilot",
			Configured: schemas["pilot"] != nil,
		},
		{
			Label:      "Route",
			Configured: schemas["route"] != nil,
		},
		{
			Label:      "PIREP",
			Configured: schemas["pirep"] != nil,
		},
		{
			Label:      "Career Mode",
			Configured: schemas["career_mode"] != nil,
		},
	}

	configured := 0
	for _, row := range rows {
		if row.Configured {
			configured++
		}
	}

	return datasourceStatusCardView{
		Rows:          rows,
		Configured:    configured,
		Total:         len(rows),
		AllConfigured: configured == len(rows),
	}
}
