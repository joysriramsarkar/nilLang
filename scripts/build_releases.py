import os
import sys
import subprocess
import tarfile
import zipfile
import hashlib
import shutil

VERSION = "0.1.0"
REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DIST_DIR = os.path.join(REPO_ROOT, "dist")

PLATFORMS = [
    {"os": "linux", "arch": "amd64", "ext": "", "archive": "tar.gz"},
    {"os": "linux", "arch": "arm64", "ext": "", "archive": "tar.gz"},
    {"os": "darwin", "arch": "amd64", "ext": "", "archive": "tar.gz"},
    {"os": "darwin", "arch": "arm64", "ext": "", "archive": "tar.gz"},
    {"os": "windows", "arch": "amd64", "ext": ".exe", "archive": "zip"},
]

COMMANDS = ["nil", "nilc", "nilpkg", "nilpkg-server", "nilkey", "softbusd"]

def build_all():
    os.makedirs(DIST_DIR, exist_ok=True)
    checksums = []

    print(f"🚀 Building Nilang v{VERSION} for all platforms...")

    for plat in PLATFORMS:
        os_name = plat["os"]
        arch = plat["arch"]
        ext = plat["ext"]
        archive_type = plat["archive"]

        print(f"\n📦 Building target: {os_name}/{arch}...")
        stage_dir = os.path.join(DIST_DIR, f"nilang-v{VERSION}-{os_name}-{arch}")
        os.makedirs(stage_dir, exist_ok=True)

        # Build each command
        env = os.environ.copy()
        env["GOOS"] = os_name
        env["GOARCH"] = arch
        env["CGO_ENABLED"] = "0"

        for cmd in COMMANDS:
            bin_name = f"{cmd}{ext}"
            out_path = os.path.join(stage_dir, bin_name)
            cmd_pkg = f"./cmd/{cmd}"
            res = subprocess.run(["go", "build", "-o", out_path, cmd_pkg], cwd=REPO_ROOT, env=env)
            if res.returncode != 0:
                print(f"❌ Failed to build {cmd} for {os_name}/{arch}")
                sys.exit(1)
            print(f"   ✓ {bin_name}")

        # Copy README and LICENSE
        shutil.copy(os.path.join(REPO_ROOT, "README.md"), stage_dir)
        shutil.copy(os.path.join(REPO_ROOT, "LICENSE"), stage_dir)

        # Create archive
        archive_name = f"nilang-v{VERSION}-{os_name}-{arch}.{archive_type}"
        archive_path = os.path.join(DIST_DIR, archive_name)

        if archive_type == "tar.gz":
            with tarfile.open(archive_path, "w:gz") as tar:
                for item in os.listdir(stage_dir):
                    tar.add(os.path.join(stage_dir, item), arcname=item)
        elif archive_type == "zip":
            with zipfile.ZipFile(archive_path, "w", zipfile.ZIP_DEFLATED) as zipf:
                for item in os.listdir(stage_dir):
                    zipf.write(os.path.join(stage_dir, item), arcname=item)

        # Calculate checksum
        with open(archive_path, "rb") as f:
            h = hashlib.sha256(f.read()).hexdigest()
        checksums.append(f"{h}  {archive_name}")
        print(f"✅ Created {archive_name} (SHA256: {h[:16]}...)")

        # Cleanup stage dir
        shutil.rmtree(stage_dir)

    # Write SHA256SUMS
    sums_path = os.path.join(DIST_DIR, "SHA256SUMS")
    with open(sums_path, "w") as f:
        f.write("\n".join(checksums) + "\n")
    print(f"\n📄 Saved checksums to {sums_path}")
    print("🎉 All releases generated successfully in dist/!")

if __name__ == "__main__":
    build_all()
