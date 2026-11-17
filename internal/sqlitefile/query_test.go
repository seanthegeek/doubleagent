package sqlitefile

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
)

// tableOf opens a table of a test database.
func tableOf(t *testing.T, database string, name string) *Table {
	t.Helper()
	table, err := openTestdata(t, database).Table(name)
	if err != nil {
		t.Fatal(err)
	}
	return table
}

// rowids returns the rowids of rows.
func rowids(rows []Row) []int64 {
	var ids []int64
	for _, row := range rows {
		ids = append(ids, row.Rowid)
	}
	return ids
}

func TestTableLookupMatchesSQL(t *testing.T) {
	database := openTestdata(t, "schema.db")
	if _, err := database.Table("absent"); !errors.Is(err, ErrNoTable) {
		t.Errorf("Table(absent) error = %v, want ErrNoTable", err)
	}
	table, err := database.Table("MIXED")
	if err != nil || table.Name() != "mixed" {
		t.Fatalf("Table(MIXED) = %v, %v; want the table mixed", table, err)
	}
	names, err := database.TableNames()
	if err != nil || !names["search"] || !names["search_content"] || names["MIXED"] {
		t.Errorf("TableNames() = %v, %v; want exact names, virtual and shadow tables included", names, err)
	}
	computed := tableOf(t, "schema.db", "computed")
	if computed.Has("b") || !computed.Has("A") {
		t.Error("Has() should list stored columns in any case and leave generated ones out")
	}
	if _, err := computed.Select("a", "c").All(); err != nil {
		t.Errorf("selecting the stored columns of a table with a computed one: %v", err)
	}
}

func TestQueryBuildingReadsLikeSQL(t *testing.T) {
	mixed := tableOf(t, "schema.db", "mixed")
	cases := []struct {
		name  string
		query Query
		want  []int64
	}{
		{"rows come in rowid order", mixed.Select(), []int64{1, 2, 3, 4, 5, 6, 7, 8}},
		{"a limit keeps the first rows", mixed.Select().Limit(3), []int64{1, 2, 3}},
		{"a zero limit keeps none", mixed.Select().Limit(0), nil},
		{"several Where functions must all hold", mixed.Select().
			Where(func(row Row) bool { return row.Equals("z", IntValue(5)) }).
			Where(func(row Row) bool { return row.Rowid > 1 }), []int64{2, 6, 8}},
		{"a descending key reverses the order, ties in rowid order",
			mixed.Select().OrderBy("z DESC"), []int64{7, 1, 2, 6, 8, 3, 5, 4}},
		{"an order and a limit take the first rows sorted", mixed.Select().OrderBy("rowid desc").Limit(2),
			[]int64{8, 7}},
		{"keys may be listed in one string as in SQL", mixed.Select().OrderBy("z desc, rowid DESC"),
			[]int64{7, 8, 6, 2, 1, 3, 5, 4}},
		{"a query is a value: Where on a copy leaves the original alone",
			func() Query {
				base := mixed.Select().Limit(2)
				_ = base.Where(func(Row) bool { return false })
				return base
			}(), []int64{1, 2}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			rows, err := test.query.All()
			if err != nil {
				t.Fatal(err)
			}
			if got := rowids(rows); !slices.Equal(got, test.want) {
				t.Errorf("rowids = %v, want %v", got, test.want)
			}
		})
	}
}

func TestQueryErrorsNameTheProblem(t *testing.T) {
	mixed := tableOf(t, "schema.db", "mixed")
	keyed := tableOf(t, "schema.db", "keyed")
	cases := []struct {
		name  string
		query Query
		want  string
	}{
		{"an unknown selected column", mixed.Select("nope"), "no such column: nope"},
		{"an unknown sort column", mixed.Select().OrderBy("nope"), "no such column: nope"},
		{"a sort key with a bad direction", mixed.Select().OrderBy("x sideways"), "cannot sort by"},
		{"an empty sort key", mixed.Select().OrderBy("x,"), "cannot sort by"},
		{"a rowid sort in a WITHOUT ROWID table", keyed.Select().OrderBy("rowid"), "no such column: rowid"},
		{"a rowid selection in a WITHOUT ROWID table", keyed.Select("rowid"), "no such column: rowid"},
		{"a virtual generated column", tableOf(t, "schema.db", "computed").Select(), "computed when read"},
		{"a query of no table", Query{}, "no table"},
	}
	for _, test := range cases {
		t.Run(test.name+" is an error", func(t *testing.T) {
			_, err := test.query.All()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("All() error = %v, want one containing %q", err, test.want)
			}
		})
	}
}

func TestEachAndFirstStopWhereTheyShould(t *testing.T) {
	mixed := tableOf(t, "schema.db", "mixed")
	stop := errors.New("enough")
	seen := 0
	err := mixed.Select().Each(func(Row) error {
		if seen++; seen == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || seen != 2 {
		t.Errorf("Each() = %v after %d rows, want the visitor's error after 2", err, seen)
	}
	row, found, err := mixed.Select("y").OrderBy("y").First()
	if err != nil || !found || !row.Get("y").IsNull() || row.Has("x") {
		t.Errorf("First() = %v, %v, %v; want the NULL y first and only y selected", row, found, err)
	}
	_, found, err = mixed.Select().Where(func(Row) bool { return false }).First()
	if err != nil || found {
		t.Errorf("First() of no rows = %v, %v; want not found", found, err)
	}
}

func TestRowsReadNamesAndValues(t *testing.T) {
	odd := tableOf(t, "schema.db", `odd "name" here`)
	rows, err := odd.Select("[a b]", "C D", "rowid").Compute("twice", func(row Row) Value {
		return IntValue(2 * row.Get("c d").Int)
	}).All()
	if err == nil {
		t.Fatalf("a bracketed name should not match a column: %v", rows)
	}
	rows, err = odd.Select("a b", "C D", "rowid").Compute("twice", func(row Row) Value {
		return IntValue(2 * row.Get("c d").Int)
	}).All()
	if err != nil || len(rows) != 1 {
		t.Fatalf("All() = %d rows, %v", len(rows), err)
	}
	row := rows[0]
	if got := strings.Join(row.Columns(), "|"); got != "a b|C D|rowid|twice" {
		t.Errorf("Columns() = %q, want the names as selected", got)
	}
	if row.Get("C D").Int != 7 || row.Get("ROWID").Int != 1 || row.Get("twice").Int != 14 {
		t.Errorf("values = %v, want 7, rowid 1 and 14", row.Values())
	}
	if !row.Get("i").IsNull() || row.Has("i") {
		t.Error("a column not selected should read as NULL and be absent")
	}
}

func TestLookupComparesAsAJoinDoes(t *testing.T) {
	people := tableOf(t, "schema.db", "people")
	mixed := tableOf(t, "schema.db", "mixed")
	byName, err := people.Select("n").Lookup("name")
	if err != nil {
		t.Fatal(err)
	}
	probes, err := people.Select("name", "t").All()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(byName.Find(probes[0], "name")); got != 2 {
		t.Errorf("a NOCASE key found %d rows for bob, want bob and BOB", got)
	}
	if got := byName.Find(probes[4], "name"); got != nil {
		t.Errorf("a NULL key found %d rows, want none", len(got))
	}
	byZ, err := mixed.Select().Lookup("z")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := mixed.Select("y").All()
	if err != nil {
		t.Fatal(err)
	}
	// y is TEXT and z INTEGER, so the text "5" and "5.0" match the integer 5.
	if got := rowids(byZ.Find(rows[0], "y")); !slices.Equal(got, []int64{1, 2, 6, 8}) {
		t.Errorf("numeric join of y = '5' found rowids %v, want 1, 2, 6, 8", got)
	}
	if got := rowids(byZ.Find(rows[5], "y")); !slices.Equal(got, []int64{1, 2, 6, 8}) {
		t.Errorf("numeric join of y = '5.0' found rowids %v, want 1, 2, 6, 8", got)
	}
	if got := byZ.Find(rows[0], "absent"); got != nil {
		t.Errorf("a probe of a missing column found %d rows", len(got))
	}
	if _, err := mixed.Select().Lookup("absent"); err == nil {
		t.Error("Lookup of a missing column should fail")
	}
}

func TestNumberTextIsSQLiteText(t *testing.T) {
	// From SQLite 3.53.4: SELECT CAST(x AS TEXT).
	cases := []struct {
		number float64
		want   string
	}{
		{0.30000000000000004, "0.30000000000000004"}, {1e15, "1000000000000000.0"}, {1e16, "10000000000000000.0"},
		{1e17, "1.0e+17"}, {123456789.123456789, "123456789.12345679"}, {1e100, "1.0e+100"},
		{5e-324, "4.9406564584124654e-324"}, {math.MaxFloat64, "1.7976931348623157e+308"},
		{math.Inf(1), "Inf"}, {math.Inf(-1), "-Inf"}, {-1.5, "-1.5"}, {1e14 + 0.5, "100000000000000.5"},
		{1e-4, "0.0001"}, {9.5e-5, "9.5e-05"}, {1e-5, "1.0e-05"}, {0.0125, "0.0125"},
		{1.2345678901234568e16, "12345678901234568.0"}, {math.Copysign(0, -1), "0.0"},
		{1.0000000000000002, "1.0000000000000002"}, {1.5e20, "1.5e+20"}, {100, "100.0"},
	}
	for _, test := range cases {
		t.Run("the float "+test.want, func(t *testing.T) {
			if got := numberText(FloatValue(test.number)); got != test.want {
				t.Errorf("numberText(%v) = %q, want %q", test.number, got, test.want)
			}
		})
	}
}

func TestNumericTextFollowsSQLite(t *testing.T) {
	// From SQLite 3.53.4: the value a NUMERIC column stores for the text.
	cases := []struct {
		text string
		want string
	}{
		{" 5", "int:5"}, {"\t5\n", "int:5"}, {"\v5", "int:5"}, {"5x", "text"}, {"1e3", "int:1000"},
		{"3.0e+5", "int:300000"}, {"1.5", "float:3ff8000000000000"}, {"0x10", "text"},
		{"9223372036854775808", "float:43e0000000000000"}, {"-9223372036854775808", "int:-9223372036854775808"},
		{"1e400", "float:7ff0000000000000"}, {"nan", "text"}, {"inf", "text"}, {"+5", "int:5"},
		{"+-5", "text"}, {".5", "float:3fe0000000000000"}, {"5.", "int:5"}, {"1e", "text"}, {"", "text"},
		{"-0.0", "int:0"}, {"4.0000000000000001", "int:4"}, {"1.2e18", "int:1200000000000000000"},
		{"5\x00", "int:5"}, {"١", "text"},
	}
	for _, test := range cases {
		t.Run("text "+test.text, func(t *testing.T) {
			value := applyAffinity(TextValue(test.text), NumericAffinity)
			got := describeValue(value)
			if value.Type == Text {
				got = "text"
			}
			if got != test.want {
				t.Errorf("NUMERIC %q = %s, want %s", test.text, got, test.want)
			}
		})
	}
}

func TestValuesGiveTheirRowText(t *testing.T) {
	cases := []struct {
		value  Value
		str    string
		truthy bool
	}{
		{Value{}, "", false}, {IntValue(0), "0", false}, {IntValue(-3), "-3", true},
		{FloatValue(1e16), "1e+16", true}, {FloatValue(0), "0", false}, {TextValue(""), "", false},
		{TextValue("é"), "é", true}, {BlobValue([]byte("a'b\x00\xff")), `x'612762` + `00ff'`, true},
		{BlobValue(nil), "x''", false},
	}
	for _, test := range cases {
		t.Run("the text of "+test.str, func(t *testing.T) {
			if got := test.value.String(); got != test.str {
				t.Errorf("String() = %q, want %q", got, test.str)
			}
			if got := !test.value.IsEmpty(); got != test.truthy {
				t.Errorf("not IsEmpty() = %v, want %v", got, test.truthy)
			}
		})
	}
}

func TestLikeBacktracksOverPercent(t *testing.T) {
	cases := []struct {
		text, pattern string
		want          bool
	}{
		{"abcabc", "%bc", true}, {"abcabd", "%b%c%d", true}, {"aXbYc", "a%b%c", true},
		{"", "%%", true}, {"a", "%_%_%", false}, {"ab", "%_%_%", true}, {"Straße", "STRA%E", true},
		{"straße", "STRASSE", false}, {"x", "", false}, {"", "", true},
	}
	for _, test := range cases {
		t.Run(test.text+" LIKE "+test.pattern, func(t *testing.T) {
			if got := Like(TextValue(test.text), test.pattern); got != test.want {
				t.Errorf("Like(%q, %q) = %v, want %v", test.text, test.pattern, got, test.want)
			}
		})
	}
}

func TestDamageEndsAQueryAfterTheRowsBeforeIt(t *testing.T) {
	main := testdataBytes(t, "deep.db")
	database := openOrFail(t, main, nil, nil)
	leaves := leafPages(t, database, rootOf(t, database, "t"))
	damaged := append([]byte(nil), main...)
	damaged[int64(leaves[3]-1)*int64(database.PageSize())] = 0x07
	table, err := openOrFail(t, damaged, nil, nil).Table("t")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := table.Select().All()
	if len(rows) == 0 || !errors.Is(err, ErrMalformed) {
		t.Errorf("an unordered query = %d rows, %v; want the rows before the damage and ErrMalformed",
			len(rows), err)
	}
	rows, err = table.Select().OrderBy("a").All()
	if len(rows) != 0 || !errors.Is(err, ErrMalformed) {
		t.Errorf("a sorted query = %d rows, %v; want none and ErrMalformed", len(rows), err)
	}
}

func FuzzTables(fuzz *testing.F) {
	for _, name := range []string{"schema.db", "sites.db", "shapes.db", "page_512.db"} {
		fuzz.Add(testdataBytes(fuzz, name))
	}
	fuzz.Fuzz(func(_ *testing.T, main []byte) {
		database, err := OpenBytes(main, nil, nil)
		if err != nil {
			return
		}
		names, _ := database.TableNames()
		for name := range names {
			table, err := database.Table(name)
			if err != nil {
				continue
			}
			_, _ = table.Select().OrderBy("rowid").All()
			for _, column := range table.Columns() {
				_, _ = table.Select(column.Name).OrderBy(column.Name + " desc").All()
			}
		}
	})
}
