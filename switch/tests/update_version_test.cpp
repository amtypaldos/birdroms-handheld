#include "update/parsing.hpp"
#include <cassert>
#include <iostream>
using namespace bird;
int main() {
  using update_detail::version_parts;
  // Regression: published date releases without a build suffix are valid and
  // compare numerically.
  const std::vector<uint32_t> expected = {2026, 11, 2, 0};
  assert(version_parts("2026.11.02") == expected);
  assert(version_parts("v2026.11.02") == expected);
  assert(version_parts("2026.11.02") > version_parts("2026.11.01.9"));
  assert(version_parts("2026.11.01.10") > version_parts("2026.11.01.9"));
  assert(version_parts("2028.02.29") == version_parts("2028.02.29.0"));
  for (auto bad : {"2026.02.29", "2026.02.31.1", "2026.04.31.1", "2026.11",
                   "2026.11.01.1.2", "2026.11.01-beta"}) {
    bool rejected = false;
    try {
      version_parts(bad);
    } catch (...) {
      rejected = true;
    }
    assert(rejected);
  }
  nlohmann::json wrong = {{"available", true},
                          {"platform", "muos"},
                          {"version", "2026.11.02"},
                          {"filename", "birdroms.zip"},
                          {"size", 1024},
                          {"sha256", std::string(64, 'a')},
                          {"nro_sha256", std::string(64, 'b')},
                          {"nro_size", 256}};
  bool rejected = false;
  try {
    parse_update_info(wrong.dump());
  } catch (...) {
    rejected = true;
  }
  assert(rejected);
  std::cout
      << "Update three/four-component numeric date version tests passed\n";
}
