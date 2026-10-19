use std::env;
use std::path::PathBuf;

fn main() {
    let manifest_dir = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let lib_dir = manifest_dir.join("../c");
    let canonical_lib_dir = lib_dir.canonicalize().unwrap_or(lib_dir);

    println!("cargo:rustc-link-search=native={}", canonical_lib_dir.display());
    println!("cargo:rustc-link-lib=dylib=topospatia");
    println!("cargo:rustc-link-arg=-Wl,-rpath,{}", canonical_lib_dir.display());
    println!("cargo:rerun-if-changed={}", canonical_lib_dir.join("libtopospatia.so").display());
}
