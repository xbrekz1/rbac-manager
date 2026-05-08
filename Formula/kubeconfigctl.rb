class Kubeconfigctl < Formula
  desc "Generate kubeconfig files for rbac-manager AccessGrants"
  homepage "https://github.com/xbrekz1/rbac-manager"
  version "1.1.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/xbrekz1/rbac-manager/releases/download/v#{version}/kubeconfigctl-darwin-arm64.tar.gz"
      sha256 "2148d1624c82af14c568b3681e1949053b1c09f05ad61716e0ed59289bf5c49f"
    end
    on_intel do
      url "https://github.com/xbrekz1/rbac-manager/releases/download/v#{version}/kubeconfigctl-darwin-amd64.tar.gz"
      sha256 "a33e2c7e287d43ab9ccc379cf5cf01f675cd1ddd33ba4017e08eb26caba7f542"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/xbrekz1/rbac-manager/releases/download/v#{version}/kubeconfigctl-linux-arm64.tar.gz"
      sha256 "4cea00f16cd46860285c30c57a435281c14cdb5063549ba5a76b5adef49a9137"
    end
    on_intel do
      url "https://github.com/xbrekz1/rbac-manager/releases/download/v#{version}/kubeconfigctl-linux-amd64.tar.gz"
      sha256 "96d21716273bcd1f13144a75c1602fef9f9d29b66875b37a8292d57bad7e07f8"
    end
  end

  def install
    bin.install "kubeconfigctl"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/kubeconfigctl --version")
  end
end
