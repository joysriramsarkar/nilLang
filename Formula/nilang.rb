class Nilang < Formula
  desc "Modern declarative programming language for Alap Framework and Onuron OS"
  homepage "https://github.com/joysriramsarkar/nilLang"
  version "0.1.0"
  license "AGPL-3.0-or-later"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/joysriramsarkar/nilLang/releases/download/v0.1.0/nilang-v0.1.0-darwin-amd64.tar.gz"
      sha256 "fce51eb3abe2bdd9a95a099350798db71a46aefa89eff725834169b3f9d9976f"
    end
    if Hardware::CPU.arm?
      url "https://github.com/joysriramsarkar/nilLang/releases/download/v0.1.0/nilang-v0.1.0-darwin-arm64.tar.gz"
      sha256 "6b28c1523547501a2749f0da3e0f6a4d470477eb4b35dc89fd37f2e826982096"
    end
  end

  on_linux do
    if Hardware::CPU.intel?
      url "https://github.com/joysriramsarkar/nilLang/releases/download/v0.1.0/nilang-v0.1.0-linux-amd64.tar.gz"
      sha256 "f59a8224397e62ea0592d0505dafd130c5e8cf9311d4d6e341f90b4f4aa05f1c"
    end
    if Hardware::CPU.arm?
      url "https://github.com/joysriramsarkar/nilLang/releases/download/v0.1.0/nilang-v0.1.0-linux-arm64.tar.gz"
      sha256 "471f3a47710d746eb889d4b5402287b8fb132c7ac45b0db4265325c58e6a136b"
    end
  end

  depends_on "go" => :build

  def install
    if build.head?
      system "go", "build", "-o", bin/"nil", "./cmd/nil"
      system "go", "build", "-o", bin/"nilc", "./cmd/nilc"
      system "go", "build", "-o", bin/"nilpkg", "./cmd/nilpkg"
      system "go", "build", "-o", bin/"nilpkg-server", "./cmd/nilpkg-server"
      system "go", "build", "-o", bin/"nilkey", "./cmd/nilkey"
      system "go", "build", "-o", bin/"softbusd", "./cmd/softbusd"
    else
      bin.install "nil"
      bin.install "nilc"
      bin.install "nilpkg"
      bin.install "nilpkg-server"
      bin.install "nilkey"
      bin.install "softbusd"
    end
  end

  test do
    system "#{bin}/nil", "version"
  end
end
