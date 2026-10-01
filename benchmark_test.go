package localbase

import (
	"fmt"
	"testing"
)

func BenchmarkWrite(b *testing.B) {
	dir := b.TempDir()
	db, _ := New(dir, &Options{Logger: &noopLogger{}})
	defer db.Close()

	user := testUser{ID: 1, Name: "Bench", Age: 30, Email: "bench@test.com"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Write("bench", fmt.Sprintf("user_%d", i), user)
	}
}

func BenchmarkRead(b *testing.B) {
	dir := b.TempDir()
	db, _ := New(dir, &Options{Logger: &noopLogger{}})
	defer db.Close()

	user := testUser{ID: 1, Name: "Bench", Age: 30, Email: "bench@test.com"}
	db.Write("bench", "target", user)

	var result testUser
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Read("bench", "target", &result)
	}
}

func BenchmarkReadAll_10(b *testing.B) {
	benchmarkReadAllN(b, 10)
}

func BenchmarkReadAll_100(b *testing.B) {
	benchmarkReadAllN(b, 100)
}

func BenchmarkReadAll_1000(b *testing.B) {
	benchmarkReadAllN(b, 1000)
}

func benchmarkReadAllN(b *testing.B, n int) {
	dir := b.TempDir()
	db, _ := New(dir, &Options{Logger: &noopLogger{}})
	defer db.Close()

	for i := 0; i < n; i++ {
		db.Write("bench", fmt.Sprintf("user_%d", i), testUser{ID: i, Name: fmt.Sprintf("User%d", i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.ReadAll("bench")
	}
}

func BenchmarkWhere(b *testing.B) {
	dir := b.TempDir()
	db, _ := New(dir, &Options{Logger: &noopLogger{}})
	defer db.Close()

	for i := 0; i < 100; i++ {
		db.Write("bench", fmt.Sprintf("user_%d", i), testUser{
			ID: i, Name: fmt.Sprintf("User%d", i), Age: i, Company: "TestCorp",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Where("bench", FieldEquals("age", 50))
	}
}

func BenchmarkUpdate(b *testing.B) {
	dir := b.TempDir()
	db, _ := New(dir, &Options{Logger: &noopLogger{}})
	defer db.Close()

	db.Write("bench", "target", testUser{ID: 1, Name: "Bench", Age: 30})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Update("bench", "target", func(current []byte) (interface{}, error) {
			return testUser{ID: 1, Name: "Bench", Age: 30 + i}, nil
		})
	}
}

func BenchmarkConcurrentReads(b *testing.B) {
	dir := b.TempDir()
	db, _ := New(dir, &Options{Logger: &noopLogger{}})
	defer db.Close()

	db.Write("bench", "target", testUser{ID: 1, Name: "Bench", Age: 30})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var result testUser
		for pb.Next() {
			db.Read("bench", "target", &result)
		}
	})
}

func BenchmarkConcurrentWrites(b *testing.B) {
	dir := b.TempDir()
	db, _ := New(dir, &Options{Logger: &noopLogger{}})
	defer db.Close()

	user := testUser{ID: 1, Name: "Bench", Age: 30}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			db.Write("bench", fmt.Sprintf("user_%d", i), user)
			i++
		}
	})
}

func BenchmarkBatchWrite(b *testing.B) {
	dir := b.TempDir()
	db, _ := New(dir, &Options{Logger: &noopLogger{}})
	defer db.Close()

	resources := make(map[string]interface{})
	for i := 0; i < 10; i++ {
		resources[fmt.Sprintf("user_%d", i)] = testUser{ID: i, Name: fmt.Sprintf("User%d", i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.BatchWrite(fmt.Sprintf("bench_%d", i), resources)
	}
}
