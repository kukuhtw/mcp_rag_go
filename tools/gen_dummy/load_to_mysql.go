/*
Kompilasi manual:
  go build -o tools/gen_dummy/load_to_mysql tools/gen_dummy/load_to_mysql.go

Pakai contoh:
  ./tools/gen_dummy/load_to_mysql \
    -table wells \
    -csv tools/gen_dummy/sample_wells.csv \
    -dsn "mcpuser:secret@tcp(127.0.0.1:3306)/mcp?parseTime=true&multiStatements=true" \
    -batch 2000 -disable-fk
*/

// [FILE] tools/gen_dummy/load_to_mysql.go
package main

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

var (
	csvPath   = flag.String("csv", "tools/gen_dummy/sample_timeseries.csv", "CSV path")
	dsn       = flag.String("dsn", "root:password@tcp(127.0.0.1:3306)/mcp?parseTime=true&multiStatements=true", "MySQL DSN")
	table     = flag.String("table", "ts_value", "Target table (ts_value|prod_allocation_daily|drilling_events|hsse_incidents|work_orders|purchase_orders|ts_signal|doc_chunks|wells)")
	batchSize = flag.Int("batch", 1000, "Insert batch size")
	truncate  = flag.Bool("truncate", false, "TRUNCATE target table first")
	disableFK = flag.Bool("disable-fk", false, "Disable foreign key checks")
)

func must(err error) { if err != nil { log.Fatal(err) } }

func main() {
	flag.Parse()

	allowed := map[string]bool{
		"ts_value":              true,
		"prod_allocation_daily": true,
		"drilling_events":       true,
		"hsse_incidents":        true,
		"work_orders":           true,
		"purchase_orders":       true,
		"ts_signal":             true,
		"doc_chunks":            true,
		"wells":                 true,
	}
	if !allowed[*table] {
		log.Fatalf("unsupported table: %s", *table)
	}

	db, err := sql.Open("mysql", *dsn)
	must(err)
	defer db.Close()
	must(db.Ping())

	if *disableFK {
		_, err := db.Exec("SET FOREIGN_KEY_CHECKS = 0")
		must(err)
		defer func() {
			_, err := db.Exec("SET FOREIGN_KEY_CHECKS = 1")
			if err != nil {
				log.Printf("Error enabling FK checks: %v", err)
			}
		}()
	}

	if *truncate {
		_, err := db.Exec("TRUNCATE TABLE " + *table)
		must(err)
		log.Printf("[ok] truncated %s", *table)
		if *csvPath == "/dev/null" {
			return
		}
	}

	f, err := os.Open(*csvPath)
	must(err)
	defer f.Close()

	r := csv.NewReader(bufio.NewReader(f))
	r.FieldsPerRecord = -1

	head, err := r.Read()
	must(err)

	switch *table {
	case "ts_value":
		loadTS(db, r, head)
	case "prod_allocation_daily":
		loadDaily(db, r, head)
	case "drilling_events":
		loadDrill(db, r, head)
	case "hsse_incidents":
		loadHSSE(db, r, head)
	case "work_orders":
		loadWO(db, r, head)
	case "purchase_orders":
		loadPO(db, r, head)
	case "ts_signal":
		loadTSSignal(db, r, head)
	case "doc_chunks":
		loadDocChunks(db, r, head)
	case "wells":
		loadWells(db, r, head)
	default:
		log.Fatalf("unsupported table: %s", *table)
	}
}

/* ======================= Common Helpers ======================= */

func headerIndex(h []string) map[string]int {
	m := map[string]int{}
	for i, c := range h {
		c = strings.TrimSpace(strings.ToLower(c))
		c = strings.TrimPrefix(c, "\ufeff")
		m[c] = i
	}
	return m
}

func ensureColumns(idx map[string]int, need []string) {
	for _, c := range need {
		if _, ok := idx[c]; !ok {
			log.Fatalf("missing column %q in CSV header", c)
		}
	}
}

func readRow(r *csv.Reader) ([]string, error) {
	rec, err := r.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	return rec, nil
}

/* ======================= ts_value ======================= */

func loadTS(db *sql.DB, r *csv.Reader, head []string) {
	idx := headerIndex(head)
	need := []string{"tag_id", "ts_utc", "value", "quality"}
	ensureColumns(idx, need)

	vals := make([]interface{}, 0, *batchSize*4)
	rows := 0
	for {
		rec, err := readRow(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatal(err)
		}
		vals = append(vals, rec[idx["tag_id"]], rec[idx["ts_utc"]], rec[idx["value"]], rec[idx["quality"]])
		rows++
		if rows%*batchSize == 0 {
			flushTS(db, &vals)
		}
	}
	if len(vals) > 0 {
		flushTS(db, &vals)
	}
	log.Printf("[ok] inserted ts_value rows: ~%d", rows)
}

func flushTS(db *sql.DB, vals *[]interface{}) {
	if len(*vals) == 0 {
		return
	}
	placeholders := strings.Repeat("(?, ?, ?, ?),", len(*vals)/4)
	placeholders = strings.TrimRight(placeholders, ",")
	q := "INSERT INTO ts_value(tag_id, ts_utc, value, quality) VALUES " + placeholders +
		" ON DUPLICATE KEY UPDATE value=VALUES(value), quality=VALUES(quality)"
	_, err := db.Exec(q, *vals...)
	must(err)
	*vals = (*vals)[:0]
}

/* ======================= prod_allocation_daily ======================= */

func loadDaily(db *sql.DB, r *csv.Reader, head []string) {
	idx := headerIndex(head)
	need := []string{"date", "well_id", "gas_mmscfd"}
	ensureColumns(idx, need)

	vals := make([]interface{}, 0, *batchSize*3)
	rows := 0
	for {
		rec, err := readRow(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatal(err)
		}
		vals = append(vals, rec[idx["date"]], rec[idx["well_id"]], rec[idx["gas_mmscfd"]])
		rows++
		if rows%*batchSize == 0 {
			flushDaily(db, &vals)
		}
	}
	if len(vals) > 0 {
		flushDaily(db, &vals)
	}
	log.Printf("[ok] inserted prod_allocation_daily rows: ~%d", rows)
}

func flushDaily(db *sql.DB, vals *[]interface{}) {
	if len(*vals) == 0 {
		return
	}
	placeholders := strings.Repeat("(?, ?, ?),", len(*vals)/3)
	placeholders = strings.TrimRight(placeholders, ",")
	q := "INSERT INTO prod_allocation_daily(date, well_id, gas_mmscfd) VALUES " + placeholders +
		" ON DUPLICATE KEY UPDATE gas_mmscfd=VALUES(gas_mmscfd)"
	_, err := db.Exec(q, *vals...)
	must(err)
	*vals = (*vals)[:0]
}

/* ======================= drilling_events ======================= */

func loadDrill(db *sql.DB, r *csv.Reader, head []string) {
	idx := headerIndex(head)
	need := []string{"well_id", "event_type", "sub_cause", "start_time", "end_time", "cost_usd"}
	ensureColumns(idx, need)

	vals := make([]interface{}, 0, *batchSize*6)
	rows := 0
	for {
		rec, err := readRow(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatal(err)
		}
		vals = append(vals,
			rec[idx["well_id"]],
			rec[idx["event_type"]],
			rec[idx["sub_cause"]],
			rec[idx["start_time"]],
			rec[idx["end_time"]],
			rec[idx["cost_usd"]],
		)
		rows++
		if rows%*batchSize == 0 {
			flushDrill(db, &vals)
		}
	}
	if len(vals) > 0 {
		flushDrill(db, &vals)
	}
	log.Printf("[ok] inserted drilling_events rows: ~%d", rows)
}

func flushDrill(db *sql.DB, vals *[]interface{}) {
	if len(*vals) == 0 {
		return
	}
	placeholders := strings.Repeat("(?, ?, ?, ?, ?, ?),", len(*vals)/6)
	placeholders = strings.TrimRight(placeholders, ",")
	q := "INSERT INTO drilling_events(well_id, event_type, sub_cause, start_time, end_time, cost_usd) VALUES " + placeholders
	_, err := db.Exec(q, *vals...)
	must(err)
	*vals = (*vals)[:0]
}

/* ======================= hsse_incidents ======================= */

func loadHSSE(db *sql.DB, r *csv.Reader, head []string) {
	idx := headerIndex(head)
	need := []string{"category", "description", "event_time", "location"}
	ensureColumns(idx, need)

	vals := make([]interface{}, 0, *batchSize*4)
	rows := 0
	for {
		rec, err := readRow(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatal(err)
		}
		vals = append(vals,
			rec[idx["category"]],
			rec[idx["description"]],
			rec[idx["event_time"]],
			rec[idx["location"]],
		)
		rows++
		if rows%*batchSize == 0 {
			flushHSSE(db, &vals)
		}
	}
	if len(vals) > 0 {
		flushHSSE(db, &vals)
	}
	log.Printf("[ok] inserted hsse_incidents rows: ~%d", rows)
}

func flushHSSE(db *sql.DB, vals *[]interface{}) {
	if len(*vals) == 0 {
		return
	}
	placeholders := strings.Repeat("(?, ?, ?, ?),", len(*vals)/4)
	placeholders = strings.TrimRight(placeholders, ",")
	q := "INSERT INTO hsse_incidents(category, description, event_time, location) VALUES " + placeholders
	_, err := db.Exec(q, *vals...)
	must(err)
	*vals = (*vals)[:0]
}

/* ======================= work_orders ======================= */

func loadWO(db *sql.DB, r *csv.Reader, head []string) {
	idx := headerIndex(head)
	need := []string{"wo_id", "asset_id", "area", "priority", "status", "due_date"}
	ensureColumns(idx, need)

	vals := make([]interface{}, 0, *batchSize*6)
	rows := 0
	for {
		rec, err := readRow(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatal(err)
		}
		vals = append(vals,
			rec[idx["wo_id"]],
			rec[idx["asset_id"]],
			rec[idx["area"]],
			rec[idx["priority"]],
			rec[idx["status"]],
			rec[idx["due_date"]],
		)
		rows++
		if rows%*batchSize == 0 {
			flushWO(db, &vals)
		}
	}
	if len(vals) > 0 {
		flushWO(db, &vals)
	}
	log.Printf("[ok] inserted work_orders rows: ~%d", rows)
}

func flushWO(db *sql.DB, vals *[]interface{}) {
	if len(*vals) == 0 {
		return
	}
	placeholders := strings.Repeat("(?, ?, ?, ?, ?, ?),", len(*vals)/6)
	placeholders = strings.TrimRight(placeholders, ",")
	q := "INSERT INTO work_orders(wo_id, asset_id, area, priority, status, due_date) VALUES " + placeholders +
		" ON DUPLICATE KEY UPDATE asset_id=VALUES(asset_id), area=VALUES(area), priority=VALUES(priority), status=VALUES(status), due_date=VALUES(due_date)"
	_, err := db.Exec(q, *vals...)
	must(err)
	*vals = (*vals)[:0]
}

/* ======================= purchase_orders ======================= */

/* ======================= purchase_orders ======================= */

func loadPO(db *sql.DB, r *csv.Reader, head []string) {
	idx := headerIndex(head)
	need := []string{"po_number", "vendor", "status", "eta", "amount"} // <— amount wajib
	ensureColumns(idx, need)

	vals := make([]interface{}, 0, *batchSize*5) // <— 5 kolom
	rows := 0
	for {
		rec, err := readRow(r)
		if err != nil {
			if errors.Is(err, io.EOF) { break }
			log.Fatal(err)
		}

		amtStr := strings.TrimSpace(rec[idx["amount"]])
		amt, err := strconv.ParseInt(amtStr, 10, 64)
		if err != nil {
			// fallback aman; bisa juga log.Printf warning sesuai kebutuhan
			amt = 0
		}

		vals = append(vals,
			rec[idx["po_number"]],
			rec[idx["vendor"]],
			rec[idx["status"]],
			rec[idx["eta"]],
			amt, // <— push amount
		)
		rows++
		if rows%*batchSize == 0 {
			flushPO(db, &vals)
		}
	}
	if len(vals) > 0 {
		flushPO(db, &vals)
	}
	log.Printf("[ok] inserted purchase_orders rows: ~%d", rows)
}


func flushPO(db *sql.DB, vals *[]interface{}) {
	if len(*vals) == 0 { return }

	placeholders := strings.Repeat("(?, ?, ?, ?, ?),", len(*vals)/5) // <— 5 kolom
	placeholders = strings.TrimRight(placeholders, ",")

	q := "INSERT INTO purchase_orders(po_number, vendor, status, eta, amount) VALUES " + placeholders +
		" ON DUPLICATE KEY UPDATE vendor=VALUES(vendor), status=VALUES(status), eta=VALUES(eta), amount=VALUES(amount)" // <— upsert amount juga

	_, err := db.Exec(q, *vals...)
	must(err)
	*vals = (*vals)[:0]
}

/* ======================= ts_signal ======================= */

func loadTSSignal(db *sql.DB, r *csv.Reader, head []string) {
	idx := headerIndex(head)
	need := []string{"tag_id", "asset_id", "tag_name", "unit", "description"}
	ensureColumns(idx, need)

	vals := make([]interface{}, 0, *batchSize*5)
	rows := 0
	for {
		rec, err := readRow(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatal(err)
		}
		vals = append(vals,
			rec[idx["tag_id"]],
			rec[idx["asset_id"]],
			rec[idx["tag_name"]],
			rec[idx["unit"]],
			rec[idx["description"]],
		)
		rows++
		if rows%*batchSize == 0 {
			flushTSSignal(db, &vals)
		}
	}
	if len(vals) > 0 {
		flushTSSignal(db, &vals)
	}
	log.Printf("[ok] inserted ts_signal rows: ~%d", rows)
}

func flushTSSignal(db *sql.DB, vals *[]interface{}) {
	if len(*vals) == 0 {
		return
	}
	placeholders := strings.Repeat("(?, ?, ?, ?, ?),", len(*vals)/5)
	placeholders = strings.TrimRight(placeholders, ",")
	q := "INSERT INTO ts_signal(tag_id, asset_id, tag_name, unit, description) VALUES " + placeholders +
		" ON DUPLICATE KEY UPDATE asset_id=VALUES(asset_id), tag_name=VALUES(tag_name), unit=VALUES(unit), description=VALUES(description)"
	_, err := db.Exec(q, *vals...)
	must(err)
	*vals = (*vals)[:0]
}

/* ======================= doc_chunks ======================= */

func loadDocChunks(db *sql.DB, r *csv.Reader, head []string) {
	idx := headerIndex(head)
	need := []string{"doc_id", "title", "url", "snippet", "page_no"}
	ensureColumns(idx, need)

	vals := make([]interface{}, 0, *batchSize*5)
	rows := 0
	for {
		rec, err := readRow(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatal(err)
		}
		vals = append(vals,
			rec[idx["doc_id"]],
			rec[idx["title"]],
			rec[idx["url"]],
			rec[idx["snippet"]],
			rec[idx["page_no"]],
		)
		rows++
		if rows%*batchSize == 0 {
			flushDocChunks(db, &vals)
		}
	}
	if len(vals) > 0 {
		flushDocChunks(db, &vals)
	}
	log.Printf("[ok] inserted doc_chunks rows: ~%d", rows)
}

func flushDocChunks(db *sql.DB, vals *[]interface{}) {
	if len(*vals) == 0 {
		return
	}
	placeholders := strings.Repeat("(?, ?, ?, ?, ?),", len(*vals)/5)
	placeholders = strings.TrimRight(placeholders, ",")
	q := "INSERT INTO doc_chunks(doc_id,title,url,snippet,page_no) VALUES " + placeholders
	_, err := db.Exec(q, *vals...)
	must(err)
	*vals = (*vals)[:0]
}

/* ======================= wells ======================= */

func loadWells(db *sql.DB, r *csv.Reader, head []string) {
    idx := headerIndex(head)
    need := []string{"well_id", "name", "type", "status"}
    ensureColumns(idx, need)

    vals := make([]interface{}, 0, *batchSize*4)
    rows := 0
    for {
        rec, err := readRow(r)
        if err != nil {
            if errors.Is(err, io.EOF) { break }
            log.Fatal(err)
        }
        vals = append(vals,
            rec[idx["well_id"]],
            rec[idx["name"]],
            rec[idx["type"]],
            rec[idx["status"]],
        )
        rows++
        if rows%*batchSize == 0 { flushWells(db, &vals) }
    }
    if len(vals) > 0 { flushWells(db, &vals) }
    log.Printf("[ok] inserted wells rows: ~%d", rows)
}

func flushWells(db *sql.DB, vals *[]interface{}) {
    if len(*vals) == 0 { return }
    placeholders := strings.Repeat("(?, ?, ?, ?),", len(*vals)/4)
    placeholders = strings.TrimRight(placeholders, ",")
    q := "INSERT INTO wells(well_id, name, type, status) VALUES " + placeholders +
         " ON DUPLICATE KEY UPDATE name=VALUES(name), type=VALUES(type), status=VALUES(status)"
    _, err := db.Exec(q, *vals...)
    must(err)
    *vals = (*vals)[:0]
}

