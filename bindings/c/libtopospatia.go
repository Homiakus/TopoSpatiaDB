package main

/*
#include <stdint.h>
#include <stdlib.h>
*/
import "C"
import (
	"encoding/json"
	"sync"
	"unsafe"

	"topospatia"
)

var (
	handlesMu sync.Mutex
	handles   = make(map[uintptr]*topospatia.DB)
	nextID    uintptr = 1
)

func registerDB(db *topospatia.DB) uintptr {
	handlesMu.Lock()
	defer handlesMu.Unlock()
	id := nextID
	nextID++
	handles[id] = db
	return id
}

func getDB(id uintptr) *topospatia.DB {
	handlesMu.Lock()
	defer handlesMu.Unlock()
	return handles[id]
}

func unregisterDB(id uintptr) *topospatia.DB {
	handlesMu.Lock()
	defer handlesMu.Unlock()
	db := handles[id]
	delete(handles, id)
	return db
}

//export topospatia_open
func topospatia_open(cDataDir *C.char, maxDim C.int, stalkDim C.int, fsync C.int) uintptr {
	dataDir := C.GoString(cDataDir)
	opts := topospatia.DefaultOptions(dataDir)
	if maxDim > 0 {
		opts.MaxDim = int(maxDim)
	}
	if stalkDim > 0 {
		opts.StalkDim = int(stalkDim)
	}
	opts.Fsync = (fsync != 0)

	db, err := topospatia.Open(opts)
	if err != nil {
		return 0
	}
	return registerDB(db)
}

//export topospatia_close
func topospatia_close(handle uintptr) C.int {
	db := unregisterDB(handle)
	if db == nil {
		return -1
	}
	if err := db.Close(); err != nil {
		return -1
	}
	return 0
}

//export topospatia_add_cell
func topospatia_add_cell(handle uintptr, vertices *C.int, count C.int) *C.char {
	db := getDB(handle)
	if db == nil || count <= 0 {
		return nil
	}
	slice := unsafe.Slice(vertices, count)
	goVerts := make([]int, count)
	for i, v := range slice {
		goVerts[i] = int(v)
	}

	cell, err := db.AddCell(goVerts)
	if err != nil {
		return nil
	}
	return C.CString(cell.ID)
}

//export topospatia_compile
func topospatia_compile(handle uintptr) C.int {
	db := getDB(handle)
	if db == nil {
		return -1
	}
	db.CompileOperators()
	return 0
}

//export topospatia_put
func topospatia_put(handle uintptr, cID *C.char, cCellID *C.char, dim C.int, vector *C.double, vecLen C.int, cPayloadJSON *C.char) C.int {
	db := getDB(handle)
	if db == nil {
		return -1
	}
	id := C.GoString(cID)
	cellID := C.GoString(cCellID)
	payloadJSON := C.GoString(cPayloadJSON)

	var payload map[string]interface{}
	if payloadJSON != "" {
		_ = json.Unmarshal([]byte(payloadJSON), &payload)
	}

	var vec topospatia.StalkVector
	if vecLen > 0 && vector != nil {
		vSlice := unsafe.Slice(vector, vecLen)
		vec = make(topospatia.StalkVector, vecLen)
		for i, val := range vSlice {
			vec[i] = float64(val)
		}
	}

	rec := &topospatia.Record{
		ID:        id,
		CellID:    cellID,
		Dimension: int(dim),
		Vector:    vec,
		Payload:   payload,
	}

	if err := db.Put(rec); err != nil {
		return -1
	}
	return 0
}

//export topospatia_get
func topospatia_get(handle uintptr, cID *C.char) *C.char {
	db := getDB(handle)
	if db == nil {
		return nil
	}
	id := C.GoString(cID)
	rec, err := db.Get(id)
	if err != nil {
		return nil
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return nil
	}
	return C.CString(string(data))
}

//export topospatia_query_cell
func topospatia_query_cell(handle uintptr, cCellID *C.char) *C.char {
	db := getDB(handle)
	if db == nil {
		return nil
	}
	cellID := C.GoString(cCellID)
	recs, err := db.QueryByCell(cellID)
	if err != nil {
		return nil
	}
	data, err := json.Marshal(recs)
	if err != nil {
		return nil
	}
	return C.CString(string(data))
}

//export topospatia_diffuse
func topospatia_diffuse(handle uintptr) *C.char {
	db := getDB(handle)
	if db == nil {
		return nil
	}
	report, err := db.Diffuse()
	if err != nil {
		return nil
	}
	data, err := json.Marshal(report)
	if err != nil {
		return nil
	}
	return C.CString(string(data))
}

//export topospatia_betti
func topospatia_betti(handle uintptr, tol C.double) *C.char {
	db := getDB(handle)
	if db == nil {
		return nil
	}
	betti := db.ComputeHigherBettiNumbers(float64(tol))
	data, err := json.Marshal(betti)
	if err != nil {
		return nil
	}
	return C.CString(string(data))
}

//export topospatia_rag_diffuse
func topospatia_rag_diffuse(handle uintptr, seeds *C.int, seedCount C.int, steps C.int, alpha C.double) *C.char {
	db := getDB(handle)
	if db == nil || seedCount <= 0 {
		return nil
	}
	slice := unsafe.Slice(seeds, seedCount)
	goSeeds := make([]int, seedCount)
	for i, s := range slice {
		goSeeds[i] = int(s)
	}

	scores := db.SimplicialRAGDiffusion(goSeeds, int(steps), float64(alpha))
	data, err := json.Marshal(scores)
	if err != nil {
		return nil
	}
	return C.CString(string(data))
}

//export topospatia_free_string
func topospatia_free_string(str *C.char) {
	if str != nil {
		C.free(unsafe.Pointer(str))
	}
}

func main() {}
