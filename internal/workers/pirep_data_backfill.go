package workers

// // PirepATSynced represents a PIREP record (local definition to avoid circular dependency)
// type PirepATSynced struct {
// 	ID             string     `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
// 	ATID           string     `gorm:"column:at_id;type:varchar(20);not null"`
// 	ServerID       string     `gorm:"column:server_id;type:uuid;not null"`
// 	Route          string     `gorm:"column:route;type:text"`
// 	FlightMode     string     `gorm:"column:flight_mode;type:varchar(50)"`
// 	FlightTime     *float64   `gorm:"column:flight_time;type:numeric(10,2)"`
// 	PilotCallsign  string     `gorm:"column:pilot_callsign;type:varchar(50)"`
// 	Aircraft       string     `gorm:"column:aircraft;type:varchar(100)"`
// 	Livery         string     `gorm:"column:livery;type:varchar(100)"`
// 	RouteATID      *string    `gorm:"column:route_at_id;type:varchar(20)"`
// 	PilotATID      *string    `gorm:"column:pilot_at_id;type:varchar(20)"`
// 	ATCreatedTime  *time.Time `gorm:"column:at_created_time"`
// 	BackfillStatus int        `gorm:"column:backfill_status;type:integer;not null;default:0"`
// 	CreatedAt      time.Time  `gorm:"column:created_at;default:now()"`
// 	UpdatedAt      time.Time  `gorm:"column:updated_at;default:now()"`
// }

// // TableName specifies the table name for GORM
// func (PirepATSynced) TableName() string {
// 	return "pirep_at_synced"
// }

// type PIREPBackfill struct {
// 	RouteRepo repositories.RouteATSyncedRepo
// 	DB        *gorm.DB
// 	Cache     cache.CacheInterface
// }

// // func NewPIREPBackfill(
// // 	db *gorm.DB,
// // 	c cache.CacheInterface,
// // 	r repositories.RouteATSyncedRepo,
// // 	p pilots.Repository) *PIREPBackfill {
// // 	return &PIREPBackfill{
// // 		RouteRepo: r,
// // 		DB:        db,
// // 		Cache:     c,
// // 	}
// // }

// // RunScheduled runs the backfill job on a schedule
// func (w *PIREPBackfill) RunScheduled(ctx context.Context, interval time.Duration) {
// 	ticker := time.NewTicker(interval)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			log.Printf("PIREP backfill scheduler stopped")
// 			return
// 		case <-ticker.C:
// 			if err := w.BackfillPireps(100, 500); err != nil {
// 				log.Printf("PIREP backfill error: %v", err)
// 			}
// 		}
// 	}
// }

// func (w *PIREPBackfill) BackfillPireps(batchSize int, delayMs int) error {
// 	// start streaming the repo
// 	log.Printf("\n[BackfillPirepJob]Starting backfilling")
// 	rows, err := w.DB.WithContext(context.Background()).
// 		Model(&PirepATSynced{}).
// 		Where("backfill_status = 0").
// 		Rows()
// 	if err != nil {
// 		log.Printf("PIREP backfill query error: %v", err)
// 		return err
// 	}
// 	defer rows.Close()

// 	count := 0
// 	batchCount := 0
// 	for rows.Next() {
// 		var rec PirepATSynced
// 		if err := w.DB.ScanRows(rows, &rec); err != nil {
// 			log.Printf("PIREP backfill scan error: %v", err)
// 			return err
// 		}

// 		// Validate required IDs exist
// 		if rec.PilotATID == nil && rec.RouteATID == nil {
// 			log.Printf("PIREP backfill skipping record id=%v: missing pilot or route AT ID", rec.ID)
// 			w.DB.WithContext(context.Background()).
// 				Model(&PirepATSynced{}).
// 				Where("id = ?", rec.ID).
// 				Updates(map[string]interface{}{
// 					"backfill_status": 2,
// 				})
// 			continue
// 		}

// 		// Fetch related values
// 		var pilot *pilots.PilotATSyncedGORM
// 		var route *sync.RouteATSynced
// 		var err error

// 		// Fetch pilot only if PilotATID is present
// 		if rec.PilotATID != nil {
// 			pilot, err = w.GetCachedPilot(rec.ServerID, *rec.PilotATID)
// 			if err != nil {
// 				log.Printf("PIREP backfill error fetching pilot (serverID=%s, pilotATID=%s): %v", rec.ServerID, *rec.PilotATID, err)
// 			}
// 		}

// 		// Fetch route only if RouteATID is present
// 		if rec.RouteATID != nil {
// 			route, err = w.GetCachedRoute(rec.ServerID, *rec.RouteATID)
// 			if err != nil {
// 				log.Printf("PIREP backfill error fetching route (serverID=%s, routeATID=%s): %v", rec.ServerID, *rec.RouteATID, err)
// 			}
// 		}
// 		if pilot == nil && route == nil {
// 			log.Printf("PIREP backfill skipping record id=%v: both pilot and route not found", rec.ID)
// 			w.DB.WithContext(context.Background()).
// 				Model(&PirepATSynced{}).
// 				Where("id = ?", rec.ID).
// 				Updates(map[string]interface{}{"backfill_status": 2})
// 			continue
// 		}

// 		// Build updates map
// 		updates := map[string]interface{}{
// 			"backfill_status": 1,
// 		}
// 		if pilot != nil {
// 			updates["pilot_callsign"] = pilot.Callsign
// 		}
// 		if route != nil {
// 			updates["route"] = route.Route
// 		}

// 		// Apply update
// 		err = w.DB.WithContext(context.Background()).
// 			Model(&PirepATSynced{}).
// 			Where("id = ?", rec.ID).
// 			Updates(updates).Error
// 		if err != nil {
// 			log.Printf("PIREP backfill update error for id=%v: %v", rec.ID, err)
// 			w.DB.WithContext(context.Background()).
// 				Model(&PirepATSynced{}).
// 				Where("id = ?", rec.ID).
// 				Updates(map[string]interface{}{"backfill_status": 2})
// 		} else {
// 			pilotStr := "(missing)"
// 			routeStr := "(missing)"
// 			if pilot != nil {
// 				pilotStr = pilot.Callsign
// 			}
// 			if route != nil {
// 				routeStr = route.Route
// 			}
// 			log.Printf("PIREP backfill updated id=%v with pilot=%s, route=%s", rec.ID, pilotStr, routeStr)
// 		}

// 		count++
// 		batchCount++

// 		// Process in batches and delay between batches
// 		if batchCount >= batchSize {
// 			if delayMs > 0 {
// 				time.Sleep(time.Duration(delayMs) * time.Millisecond)
// 			}
// 			batchCount = 0
// 		}
// 	}

// 	if count > 0 {
// 		log.Printf("PIREP backfill completed: %d records processed", count)
// 	}

// 	return rows.Err()

// }

// func (w *PIREPBackfill) GetCachedPilot(sID string, atID string) (*pilots.PilotATSyncedGORM, error) {

// 	cacheKey := fmt.Sprintf("pilot:%s:%s", sID, atID)
// 	val, err := w.Cache.GetOrSet(cacheKey, 30*time.Minute, func() (any, error) {
// 		pilot, err := w.PilotRepo.FindByATID(context.Background(), sID, atID)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return pilot, nil

// 	})

// 	if err != nil {
// 		log.Printf("Pilot not found: %s", cacheKey)
// 		return nil, err
// 	}

// 	if val == nil {
// 		// Pilot not found
// 		return nil, nil
// 	}

// 	// Try direct type assertion first
// 	normalized, ok := val.(*pilots.PilotATSyncedGORM)
// 	if ok {
// 		return normalized, nil
// 	}

// 	// If that fails, try to unmarshal from the map (cache returns JSON as map)
// 	jsonBytes, err := json.Marshal(val)
// 	if err != nil {
// 		log.Printf("Unable to marshal cached pilot record: %v", err)
// 		return nil, errors.New("unable to marshal cached record")
// 	}

// 	normalized = &pilots.PilotATSyncedGORM{}
// 	if err := json.Unmarshal(jsonBytes, normalized); err != nil {
// 		log.Printf("Unable to unmarshal cached pilot record: %v", err)
// 		return nil, errors.New("unable to unmarshal cached record")
// 	}

// 	return normalized, nil

// }

// func (w *PIREPBackfill) GetCachedRoute(sID string, atID string) (*sync.RouteATSynced, error) {

// 	cacheKey := fmt.Sprintf("route:%s:%s", sID, atID)
// 	val, err := w.Cache.GetOrSet(cacheKey, 30*time.Minute, func() (any, error) {
// 		route, err := w.RouteRepo.FindByATID(context.Background(), sID, atID)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return route, nil

// 	})

// 	if err != nil {
// 		log.Printf("Route not found: %s", cacheKey)
// 		return nil, err
// 	}

// 	if val == nil {
// 		// Route not found
// 		return nil, nil
// 	}

// 	// Try direct type assertion first
// 	normalized, ok := val.(*sync.RouteATSynced)
// 	if ok {
// 		return normalized, nil
// 	}

// 	// If that fails, try to unmarshal from the map (cache returns JSON as map)
// 	jsonBytes, err := json.Marshal(val)
// 	if err != nil {
// 		log.Printf("Unable to marshal cached route record: %v", err)
// 		return nil, errors.New("unable to marshal cached record")
// 	}

// 	normalized = &sync.RouteATSynced{}
// 	if err := json.Unmarshal(jsonBytes, normalized); err != nil {
// 		log.Printf("Unable to unmarshal cached route record: %v", err)
// 		return nil, errors.New("unable to unmarshal cached record")
// 	}

// 	return normalized, nil

// }
