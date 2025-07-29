class NlToShell < Formula
  desc "Convert natural language to shell commands using LLMs"
  homepage "https://github.com/nl-to-shell/nl-to-shell"
  url "https://github.com/nl-to-shell/nl-to-shell/releases/download/0.1.0-dev/nl-to-shell-darwin-amd64.tar.gz"
  sha256 "be85267296c071ed32cf4593c5ce26de3daef8a9ab3d72efe624c6890f35f366"
  license "MIT"
  version "0.1.0-dev"

  depends_on "git"

  on_arm do
    url "https://github.com/nl-to-shell/nl-to-shell/releases/download/0.1.0-dev/nl-to-shell-darwin-arm64.tar.gz"
    sha256 "461a5d327f3a98ab8192baf24f12fe8f57cea0d48b1c5718a4bd1c7f3ea8802c"
  end

  def install
    bin.install "nl-to-shell-darwin-amd64" => "nl-to-shell" if Hardware::CPU.intel?
    bin.install "nl-to-shell-darwin-arm64" => "nl-to-shell" if Hardware::CPU.arm?
  end

  test do
    system "#{bin}/nl-to-shell", "version"
  end
end
