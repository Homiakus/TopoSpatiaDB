"""
TopoSpatiaDB Python In-Process Embedded Library.
Provides direct C-FFI bindings to libtopospatia.so for ultra-low latency,
zero-serialization embedded database operations.
"""

import ctypes
import json
import os
from typing import List, Dict, Any, Optional

def _find_library() -> str:
    # Check relative to module file and current working directory
    base_dir = os.path.dirname(os.path.abspath(__file__))
    candidates = [
        os.path.join(base_dir, "../../c/libtopospatia.so"),
        os.path.join(base_dir, "../c/libtopospatia.so"),
        os.path.join(base_dir, "libtopospatia.so"),
        os.path.abspath("bindings/c/libtopospatia.so"),
        "/usr/local/lib/libtopospatia.so",
    ]
    for c in candidates:
        if os.path.exists(c):
            return os.path.abspath(c)
    return "libtopospatia.so"

class TopoSpatiaDBError(Exception):
    pass

class DB:
    """Embedded TopoSpatia database instance."""
    
    _lib = None

    @classmethod
    def _get_lib(cls):
        if cls._lib is None:
            lib_path = _find_library()
            cls._lib = ctypes.CDLL(lib_path)
            
            # Function signatures
            cls._lib.topospatia_open.argtypes = [ctypes.c_char_p, ctypes.c_int, ctypes.c_int, ctypes.c_int]
            cls._lib.topospatia_open.restype = ctypes.c_size_t

            cls._lib.topospatia_close.argtypes = [ctypes.c_size_t]
            cls._lib.topospatia_close.restype = ctypes.c_int

            cls._lib.topospatia_add_cell.argtypes = [ctypes.c_size_t, ctypes.POINTER(ctypes.c_int), ctypes.c_int]
            cls._lib.topospatia_add_cell.restype = ctypes.c_void_p

            cls._lib.topospatia_compile.argtypes = [ctypes.c_size_t]
            cls._lib.topospatia_compile.restype = ctypes.c_int

            cls._lib.topospatia_put.argtypes = [
                ctypes.c_size_t,
                ctypes.c_char_p,
                ctypes.c_char_p,
                ctypes.c_int,
                ctypes.POINTER(ctypes.c_double),
                ctypes.c_int,
                ctypes.c_char_p
            ]
            cls._lib.topospatia_put.restype = ctypes.c_int

            cls._lib.topospatia_get.argtypes = [ctypes.c_size_t, ctypes.c_char_p]
            cls._lib.topospatia_get.restype = ctypes.c_void_p

            cls._lib.topospatia_query_cell.argtypes = [ctypes.c_size_t, ctypes.c_char_p]
            cls._lib.topospatia_query_cell.restype = ctypes.c_void_p

            cls._lib.topospatia_diffuse.argtypes = [ctypes.c_size_t]
            cls._lib.topospatia_diffuse.restype = ctypes.c_void_p

            cls._lib.topospatia_betti.argtypes = [ctypes.c_size_t, ctypes.c_double]
            cls._lib.topospatia_betti.restype = ctypes.c_void_p

            cls._lib.topospatia_rag_diffuse.argtypes = [
                ctypes.c_size_t,
                ctypes.POINTER(ctypes.c_int),
                ctypes.c_int,
                ctypes.c_int,
                ctypes.c_double
            ]
            cls._lib.topospatia_rag_diffuse.restype = ctypes.c_void_p

            cls._lib.topospatia_free_string.argtypes = [ctypes.c_void_p]
            cls._lib.topospatia_free_string.restype = None

        return cls._lib

    def __init__(self, data_dir: str, max_dim: int = 2, stalk_dim: int = 2, fsync: bool = False):
        lib = self._get_lib()
        handle = lib.topospatia_open(data_dir.encode("utf-8"), max_dim, stalk_dim, 1 if fsync else 0)
        if handle == 0:
            raise TopoSpatiaDBError(f"Failed to open TopoSpatiaDB at {data_dir}")
        self._handle = handle

    def close(self):
        if getattr(self, "_handle", 0) != 0:
            self._get_lib().topospatia_close(self._handle)
            self._handle = 0

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()

    def add_cell(self, vertices: List[int]) -> str:
        arr = (ctypes.c_int * len(vertices))(*vertices)
        res_ptr = self._get_lib().topospatia_add_cell(self._handle, arr, len(vertices))
        if not res_ptr:
            raise TopoSpatiaDBError(f"Failed to add cell with vertices {vertices}")
        cell_id = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        return cell_id

    def compile(self):
        ret = self._get_lib().topospatia_compile(self._handle)
        if ret != 0:
            raise TopoSpatiaDBError("Failed to compile simplicial operators")

    def put(self, doc_id: str, cell_id: str, dim: int = 0, vector: Optional[List[float]] = None, payload: Optional[Dict[str, Any]] = None):
        vec_ptr = None
        vec_len = 0
        if vector:
            vec_len = len(vector)
            vec_ptr = (ctypes.c_double * vec_len)(*vector)
        
        payload_str = json.dumps(payload or {})
        ret = self._get_lib().topospatia_put(
            self._handle,
            doc_id.encode("utf-8"),
            cell_id.encode("utf-8"),
            dim,
            vec_ptr,
            vec_len,
            payload_str.encode("utf-8")
        )
        if ret != 0:
            raise TopoSpatiaDBError(f"Failed to put record {doc_id}")

    def get(self, doc_id: str) -> Optional[Dict[str, Any]]:
        res_ptr = self._get_lib().topospatia_get(self._handle, doc_id.encode("utf-8"))
        if not res_ptr:
            return None
        json_str = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        return json.loads(json_str)

    def query_cell(self, cell_id: str) -> List[Dict[str, Any]]:
        res_ptr = self._get_lib().topospatia_query_cell(self._handle, cell_id.encode("utf-8"))
        if not res_ptr:
            return []
        json_str = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        return json.loads(json_str)

    def diffuse(self) -> Dict[str, Any]:
        res_ptr = self._get_lib().topospatia_diffuse(self._handle)
        if not res_ptr:
            raise TopoSpatiaDBError("Topological consensus diffusion failed")
        json_str = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        return json.loads(json_str)

    def betti_numbers(self, tol: float = 1e-5) -> Dict[str, int]:
        res_ptr = self._get_lib().topospatia_betti(self._handle, ctypes.c_double(tol))
        if not res_ptr:
            return {}
        json_str = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        return json.loads(json_str)

    def rag_diffuse(self, seed_nodes: List[int], steps: int = 3, alpha: float = 0.1) -> Dict[str, float]:
        arr = (ctypes.c_int * len(seed_nodes))(*seed_nodes)
        res_ptr = self._get_lib().topospatia_rag_diffuse(self._handle, arr, len(seed_nodes), steps, ctypes.c_double(alpha))
        if not res_ptr:
            return {}
        json_str = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        return json.loads(json_str)
