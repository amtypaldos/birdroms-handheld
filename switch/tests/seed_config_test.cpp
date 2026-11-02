#include "config/storage.hpp"
#include <cassert>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <unistd.h>
int main() {
  namespace fs = std::filesystem;
  using namespace bird;
  const auto root = fs::temp_directory_path() /
                    ("birdroms-seed-test-" + std::to_string(getpid()));
  fs::remove_all(root);
  fs::create_directories(root);
  auto seed = (root / "server.json").string();
  {
    std::ofstream f(seed);
    f << R"({"server_url":"https://example.test/"})";
  }
  auto fresh = with_seed_server(Config{}, seed);
  assert(fresh.server_url == "https://example.test" && !fresh.paired());
  assert(!fs::exists(root / "config.json") &&
         !fs::exists(root / "config.json.tmp"));
  Config paired;
  paired.server_url = "https://existing.test";
  paired.device_token = "fixture-private";
  paired.rom_base = (root / "chosen").string();
  paired.platform_dirs["NES"] = "Nintendo";
  auto preserved = with_seed_server(paired, seed);
  assert(preserved.server_url == paired.server_url &&
         preserved.device_token == paired.device_token &&
         preserved.rom_base == paired.rom_base &&
         preserved.platform_dirs == paired.platform_dirs);
  for (
      const auto &data :
      {R"({"server_url":"https://example.test","device_token":"must-not-import"})",
       R"({"server_url":"file:///private"})",
       R"({"server_url":"https://user:password@example.test"})", R"({broken)",
       R"({"server_url":null})"}) {
    {
      std::ofstream f(seed);
      f << data;
    }
    auto invalid = with_seed_server(Config{}, seed);
    assert(invalid.server_url.empty() && invalid.device_token.empty());
  }
  assert(with_seed_server(Config{}, (root / "missing").string())
             .server_url.empty());
  fs::remove_all(root);
  std::cout << "Server seed config tests passed (memory-only; token/settings "
               "preserved)\n";
}
