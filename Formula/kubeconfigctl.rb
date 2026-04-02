class Kubeconfigctl < Formula
  desc "Generate kubeconfig files for rbac-manager AccessGrants"
  homepage "https://github.com/xbrekz1/rbac-manager"
  version "1.0.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/xbrekz1/rbac-manager/releases/download/v#{version}/kubeconfigctl-darwin-arm64.tar.gz"
      sha256 "HOMEBREW_SHA256_DARWIN_ARM64"
    end
    on_intel do
      url "https://github.com/xbrekz1/rbac-manager/releases/download/v#{version}/kubeconfigctl-darwin-amd64.tar.gz"
      sha256 "HOMEBREW_SHA256_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/xbrekz1/rbac-manager/releases/download/v#{version}/kubeconfigctl-linux-arm64.tar.gz"
      sha256 "HOMEBREW_SHA256_LINUX_ARM64"
    end
    on_intel do
      url "https://github.com/xbrekz1/rbac-manager/releases/download/v#{version}/kubeconfigctl-linux-amd64.tar.gz"
      sha256 "HOMEBREW_SHA256_LINUX_AMD64"
    end
  end

  def install
    bin.install "kubeconfigctl"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/kubeconfigctl --version")
  end
end
