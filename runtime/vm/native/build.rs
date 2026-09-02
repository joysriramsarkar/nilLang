fn main() {
    // Tell cargo to link against the C library if needed
    // For now, no external dependencies needed
    
    // If targeting Android, we might need to link against Android NDK libraries
    let target = std::env::var("TARGET").unwrap_or_default();
    
    if target.contains("android") {
        println!("cargo:rustc-link-lib=log");
        println!("cargo:rustc-link-lib=android");
    }
    
    if target.contains("linux") {
        println!("cargo:rustc-link-lib=dl");
        println!("cargo:rustc-link-lib=pthread");
    }
}