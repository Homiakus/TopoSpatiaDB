"""
TopoSpatiaDB Python In-Process Embedded Library.
Provides direct C-FFI bindings to libtopospatia.so for ultra-low latency,
zero-serialization embedded database operations with dataclass ergonomics.
"""

import ctypes
import json
import os
from dataclasses import dataclass, field
from typing import List, Dict, Any, Optional, Sequence, Union

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
    """Base exception for TopoSpatiaDB operations."""
    pass


@dataclass
class Record:
    """Represents a document or entity stored in TopoSpatiaDB."""
    id: str
    cell_id: str
    dimension: int = 0
    vector: List[float] = field(default_factory=list)
    payload: Dict[str, Any] = field(default_factory=dict)
    version: int = 1
    updated_at: int = 0
    signature: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Record":
        return cls(
            id=data.get("id", ""),
            cell_id=data.get("cell_id", ""),
            dimension=data.get("dimension", 0),
            vector=data.get("vector") or [],
            payload=data.get("payload") or {},
            version=data.get("version", 1),
            updated_at=data.get("updated_at", 0),
            signature=data.get("signature", ""),
        )

    def to_dict(self) -> Dict[str, Any]:
        return {
            "id": self.id,
            "cell_id": self.cell_id,
            "dimension": self.dimension,
            "vector": self.vector,
            "payload": self.payload,
            "version": self.version,
            "updated_at": self.updated_at,
            "signature": self.signature,
        }


@dataclass
class DiffusionReport:
    """Result of discrete Hodge Laplacian consensus diffusion."""
    initial_energy: float
    final_energy: float
    iterations: int
    converged: bool
    betti_0: int
    residual_harmonic: List[float] = field(default_factory=list)
    term: int = 0
    commit_index: int = 0
    active_peers: int = 0

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "DiffusionReport":
        return cls(
            initial_energy=data.get("initial_energy", 0.0),
            final_energy=data.get("final_energy", 0.0),
            iterations=data.get("iterations", 0),
            converged=data.get("converged", False),
            betti_0=data.get("betti_0", 0),
            residual_harmonic=data.get("residual_harmonic") or [],
            term=data.get("term", 0),
            commit_index=data.get("commit_index", 0),
            active_peers=data.get("active_peers", 0),
        )


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

    def add_cell(self, vertices: Sequence[int]) -> str:
        """Add an oriented simplicial cell (e.g. [0] vertex, [0, 1] edge, [0, 1, 2] face)."""
        v_list = list(vertices)
        arr = (ctypes.c_int * len(v_list))(*v_list)
        res_ptr = self._get_lib().topospatia_add_cell(self._handle, arr, len(v_list))
        if not res_ptr:
            raise TopoSpatiaDBError(f"Failed to add cell with vertices {v_list}")
        cell_id = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        return cell_id

    def add_simplex(self, *vertices: int) -> str:
        """Ergonomic shorthand to add a simplex: db.add_simplex(0, 1, 2)"""
        return self.add_cell(list(vertices))

    def compile(self):
        """Compile Hodge Laplacian and boundary/coboundary matrices."""
        ret = self._get_lib().topospatia_compile(self._handle)
        if ret != 0:
            raise TopoSpatiaDBError("Failed to compile simplicial operators")

    def put(
        self,
        doc_or_id: Union[Record, str],
        cell_id: Optional[str] = None,
        dim: int = 0,
        vector: Optional[Sequence[float]] = None,
        payload: Optional[Dict[str, Any]] = None,
    ):
        """Store a record or Record instance attached to a simplicial cell stalk."""
        if isinstance(doc_or_id, Record):
            doc_id = doc_or_id.id
            c_id = doc_or_id.cell_id
            d = doc_or_id.dimension
            vec = doc_or_id.vector
            pl = doc_or_id.payload
        else:
            doc_id = doc_or_id
            c_id = cell_id or ""
            d = dim
            vec = list(vector) if vector is not None else None
            pl = payload or {}

        vec_ptr = None
        vec_len = 0
        if vec:
            vec_len = len(vec)
            vec_ptr = (ctypes.c_double * vec_len)(*vec)
        
        payload_str = json.dumps(pl)
        ret = self._get_lib().topospatia_put(
            self._handle,
            doc_id.encode("utf-8"),
            c_id.encode("utf-8"),
            d,
            vec_ptr,
            vec_len,
            payload_str.encode("utf-8")
        )
        if ret != 0:
            raise TopoSpatiaDBError(f"Failed to put record {doc_id}")

    def get(self, doc_id: str) -> Optional[Record]:
        """Retrieve a document by ID as a typed Record."""
        res_ptr = self._get_lib().topospatia_get(self._handle, doc_id.encode("utf-8"))
        if not res_ptr:
            return None
        json_str = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        data = json.loads(json_str)
        return Record.from_dict(data)

    def query_cell(self, cell_id: str) -> List[Record]:
        """Query all documents attached to a given cell."""
        res_ptr = self._get_lib().topospatia_query_cell(self._handle, cell_id.encode("utf-8"))
        if not res_ptr:
            return []
        json_str = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        raw_list = json.loads(json_str)
        return [Record.from_dict(r) for r in raw_list]

    def diffuse(self) -> DiffusionReport:
        """Execute discrete Hodge Laplacian heat diffusion consensus."""
        res_ptr = self._get_lib().topospatia_diffuse(self._handle)
        if not res_ptr:
            raise TopoSpatiaDBError("Topological consensus diffusion failed")
        json_str = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        return DiffusionReport.from_dict(json.loads(json_str))

    def betti_numbers(self, tol: float = 1e-5) -> Dict[str, int]:
        """Compute topological invariants (Betti numbers)."""
        res_ptr = self._get_lib().topospatia_betti(self._handle, ctypes.c_double(tol))
        if not res_ptr:
            return {}
        json_str = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        return json.loads(json_str)

    def rag_diffuse(self, seed_nodes: Sequence[int], steps: int = 3, alpha: float = 0.1) -> Dict[str, float]:
        """Simplicial RAG topological relevance propagation wave."""
        s_list = list(seed_nodes)
        arr = (ctypes.c_int * len(s_list))(*s_list)
        res_ptr = self._get_lib().topospatia_rag_diffuse(self._handle, arr, len(s_list), steps, ctypes.c_double(alpha))
        if not res_ptr:
            return {}
        json_str = ctypes.string_at(res_ptr).decode("utf-8")
        self._get_lib().topospatia_free_string(res_ptr)
        return json.loads(json_str)
