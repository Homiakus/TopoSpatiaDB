use std::collections::HashMap;
use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_double, c_int};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum TopoSpatiaError {
    #[error("Database open failed at path: {0}")]
    OpenFailed(String),
    #[error("Operation failed: {0}")]
    OperationFailed(String),
    #[error("Serialization error: {0}")]
    SerdeError(#[from] serde_json::Error),
    #[error("Null string received from Cgo")]
    NullString,
}

extern "C" {
    fn topospatia_open(data_dir: *const c_char, max_dim: c_int, stalk_dim: c_int, fsync: c_int) -> usize;
    fn topospatia_close(handle: usize) -> c_int;
    fn topospatia_add_cell(handle: usize, vertices: *const c_int, count: c_int) -> *mut c_char;
    fn topospatia_compile(handle: usize) -> c_int;
    fn topospatia_put(
        handle: usize,
        id: *const c_char,
        cell_id: *const c_char,
        dim: c_int,
        vector: *const c_double,
        vec_len: c_int,
        payload_json: *const c_char,
    ) -> c_int;
    fn topospatia_get(handle: usize, id: *const c_char) -> *mut c_char;
    fn topospatia_query_cell(handle: usize, cell_id: *const c_char) -> *mut c_char;
    fn topospatia_diffuse(handle: usize) -> *mut c_char;
    fn topospatia_betti(handle: usize, tol: c_double) -> *mut c_char;
    fn topospatia_rag_diffuse(handle: usize, seeds: *const c_int, seed_count: c_int, steps: c_int, alpha: c_double) -> *mut c_char;
    fn topospatia_free_string(str: *mut c_char);
}

fn consume_c_string(ptr: *mut c_char) -> Result<String, TopoSpatiaError> {
    if ptr.is_null() {
        return Err(TopoSpatiaError::NullString);
    }
    unsafe {
        let c_str = CStr::from_ptr(ptr);
        let s = c_str.to_string_lossy().into_owned();
        topospatia_free_string(ptr);
        Ok(s)
    }
}

/// Embedded TopoSpatiaDB instance.
pub struct DB {
    handle: usize,
}

unsafe impl Send for DB {}
unsafe impl Sync for DB {}

impl DB {
    /// Opens or initializes an embedded TopoSpatia database at `data_dir`.
    pub fn open(data_dir: &str, max_dim: i32, stalk_dim: i32, fsync: bool) -> Result<Self, TopoSpatiaError> {
        let c_dir = CString::new(data_dir).map_err(|e| TopoSpatiaError::OperationFailed(e.to_string()))?;
        let handle = unsafe {
            topospatia_open(c_dir.as_ptr(), max_dim as c_int, stalk_dim as c_int, if fsync { 1 } else { 0 })
        };
        if handle == 0 {
            return Err(TopoSpatiaError::OpenFailed(data_dir.to_string()));
        }
        Ok(Self { handle })
    }

    /// Opens database with default options (max_dim=2, stalk_dim=2, fsync=false).
    pub fn open_default(data_dir: &str) -> Result<Self, TopoSpatiaError> {
        Self::open(data_dir, 2, 2, false)
    }

    /// Inserts a simplex cell (e.g. vertices `[0]`, `[0, 1]`, `[0, 1, 2]`) and returns its canonical ID.
    pub fn add_cell(&self, vertices: &[i32]) -> Result<String, TopoSpatiaError> {
        let ptr = unsafe {
            topospatia_add_cell(self.handle, vertices.as_ptr() as *const c_int, vertices.len() as c_int)
        };
        consume_c_string(ptr)
    }

    /// Compiles coboundary operators and Hodge Laplacians.
    pub fn compile(&self) -> Result<(), TopoSpatiaError> {
        let ret = unsafe { topospatia_compile(self.handle) };
        if ret != 0 {
            return Err(TopoSpatiaError::OperationFailed("Compile operators failed".into()));
        }
        Ok(())
    }

    /// Writes or updates a document inside a cell stalk.
    pub fn put<T: serde::Serialize>(
        &self,
        id: &str,
        cell_id: &str,
        dim: i32,
        vector: &[f64],
        payload: &T,
    ) -> Result<(), TopoSpatiaError> {
        let c_id = CString::new(id).unwrap();
        let c_cell_id = CString::new(cell_id).unwrap();
        let payload_json = serde_json::to_string(payload)?;
        let c_payload = CString::new(payload_json).unwrap();

        let ret = unsafe {
            topospatia_put(
                self.handle,
                c_id.as_ptr(),
                c_cell_id.as_ptr(),
                dim as c_int,
                vector.as_ptr() as *const c_double,
                vector.len() as c_int,
                c_payload.as_ptr(),
            )
        };
        if ret != 0 {
            return Err(TopoSpatiaError::OperationFailed(format!("Put failed for {}", id)));
        }
        Ok(())
    }

    /// Retrieves a document by ID.
    pub fn get<T: serde::de::DeserializeOwned>(&self, id: &str) -> Result<Option<T>, TopoSpatiaError> {
        let c_id = CString::new(id).unwrap();
        let ptr = unsafe { topospatia_get(self.handle, c_id.as_ptr()) };
        if ptr.is_null() {
            return Ok(None);
        }
        let json_str = consume_c_string(ptr)?;
        let item: T = serde_json::from_str(&json_str)?;
        Ok(Some(item))
    }

    /// Queries all records attached to a cell ID.
    pub fn query_cell<T: serde::de::DeserializeOwned>(&self, cell_id: &str) -> Result<Vec<T>, TopoSpatiaError> {
        let c_cell = CString::new(cell_id).unwrap();
        let ptr = unsafe { topospatia_query_cell(self.handle, c_cell.as_ptr()) };
        if ptr.is_null() {
            return Ok(Vec::new());
        }
        let json_str = consume_c_string(ptr)?;
        let items: Vec<T> = serde_json::from_str(&json_str)?;
        Ok(items)
    }

    /// Diffuses states across the complex to eliminate H^1 replica disagreements.
    pub fn diffuse(&self) -> Result<serde_json::Value, TopoSpatiaError> {
        let ptr = unsafe { topospatia_diffuse(self.handle) };
        let json_str = consume_c_string(ptr)?;
        let report: serde_json::Value = serde_json::from_str(&json_str)?;
        Ok(report)
    }

    /// Computes higher Betti numbers H^0, H^1, ..., H^maxDim.
    pub fn betti_numbers(&self, tol: f64) -> Result<HashMap<String, i32>, TopoSpatiaError> {
        let ptr = unsafe { topospatia_betti(self.handle, tol as c_double) };
        let json_str = consume_c_string(ptr)?;
        let betti: HashMap<String, i32> = serde_json::from_str(&json_str)?;
        Ok(betti)
    }

    /// Simplicial RAG Diffusion for spreading query activations to rank context.
    pub fn rag_diffuse(&self, seeds: &[i32], steps: i32, alpha: f64) -> Result<HashMap<String, f64>, TopoSpatiaError> {
        let ptr = unsafe {
            topospatia_rag_diffuse(
                self.handle,
                seeds.as_ptr() as *const c_int,
                seeds.len() as c_int,
                steps as c_int,
                alpha as c_double,
            )
        };
        let json_str = consume_c_string(ptr)?;
        let map: HashMap<String, f64> = serde_json::from_str(&json_str)?;
        Ok(map)
    }
}

impl Drop for DB {
    fn drop(&mut self) {
        if self.handle != 0 {
            unsafe {
                topospatia_close(self.handle);
            }
            self.handle = 0;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_rust_embedded_db() {
        let tmp_dir = std::env::temp_dir().join(format!("topospatia_rs_{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp_dir);
        let path_str = tmp_dir.to_str().unwrap();

        {
            let db = DB::open_default(path_str).expect("Failed to open TopoSpatiaDB in Rust");
            let c0 = db.add_cell(&[0]).expect("Add cell 0");
            let _c1 = db.add_cell(&[1]).expect("Add cell 1");
            let _ = db.add_cell(&[0, 1]).expect("Add cell 0-1");
            db.compile().expect("Compile operators");

            let payload = serde_json::json!({ "engine": "rust", "status": "active" });
            db.put("rs-node-0", &c0, 0, &[12.5, 3.5], &payload).expect("Put record");

            let doc: Option<serde_json::Value> = db.get("rs-node-0").expect("Get record");
            assert!(doc.is_some());
            let doc = doc.unwrap();
            assert_eq!(doc["id"], "rs-node-0");

            let betti = db.betti_numbers(1e-5).expect("Betti numbers");
            assert_eq!(*betti.get("0").unwrap(), 2); // 1 component * stalkDim 2

            let diff = db.diffuse().expect("Consensus diffuse");
            assert!(diff["converged"].as_bool().unwrap());
        }

        let _ = std::fs::remove_dir_all(&tmp_dir);
    }
}
