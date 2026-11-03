#include "config/storage.hpp"
#include <cassert>
#include <cerrno>
#include <cstdio>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <string>
#include <unistd.h>

namespace fs = std::filesystem;
static std::string primary;
static bool refuse_overwrite = true, fail_install = false, fail_restore = false;
static int overwrite_attempts = 0;
extern "C" int __real_rename(const char *, const char *);
// Reproduce the libnx filesystem's non-POSIX rename behavior on Linux.
extern "C" int __wrap_rename(const char *from, const char *to) {
  const std::string source(from), target(to);
  if (refuse_overwrite && fs::exists(target)) {
    ++overwrite_attempts;
    errno = EEXIST;
    return -1;
  }
  if (fail_install && target == primary && source == primary + ".tmp") {
    errno = EIO;
    return -1;
  }
  if (fail_restore && target == primary && source == primary + ".bak") {
    errno = EIO;
    return -1;
  }
  return __real_rename(from, to);
}

template <class F> void rejects(F fn) {
  bool rejected = false;
  try {
    fn();
  } catch (const std::exception &) {
    rejected = true;
  }
  assert(rejected);
}
int main() {
  using namespace bird;
  const auto root = fs::temp_directory_path() /
                    ("birdroms-config-replace-" + std::to_string(getpid()));
  fs::remove_all(root);
  fs::create_directories(root);
  primary = (root / "config.json").string();
  Config initial;
  initial.server_url = "https://example.test";
  initial.rom_base = (root / "roms").string();
  // First-run URL entry and pending/cancelled pairing cannot create config,
  // staging files or even the containing directory.
  const auto fresh = (root / "fresh" / "config.json").string();
  rejects([&] { save_paired_config(fresh, initial); });
  assert(!fs::exists(root / "fresh"));
  save_config(primary, initial);
  Config approved = initial;
  approved.device_token = "fixture-only-approved-token";
  save_paired_config(primary, approved);
  assert(overwrite_attempts > 0);
  assert(load_config(primary).device_token == approved.device_token);
  assert(!fs::exists(primary + ".tmp") && !fs::exists(primary + ".bak"));
  // Re-pairing after revocation also leaves the stored config untouched until
  // a replacement token has actually arrived.
  rejects([&] { save_paired_config(primary, initial); });
  assert(load_config(primary).device_token == approved.device_token);
  assert(!fs::exists(primary + ".tmp") && !fs::exists(primary + ".bak"));

  // A final rename failure restores the previous working configuration.
  fail_install = true;
  rejects([&] { save_config(primary, initial); });
  assert(load_config(primary).device_token == approved.device_token);
  assert(fs::exists(primary + ".tmp"));
  fail_install = false;
  save_config(primary, approved);

  // Power loss after backing up the old file: the complete new token wins.
  fs::copy_file(primary, primary + ".tmp");
  assert(__real_rename(primary.c_str(), (primary + ".bak").c_str()) == 0);
  assert(load_config(primary).device_token == approved.device_token);
  assert(fs::exists(primary) && !fs::exists(primary + ".bak"));

  // A truncated staged file recovers the valid previous file instead.
  assert(__real_rename(primary.c_str(), (primary + ".bak").c_str()) == 0);
  {
    std::ofstream out(primary + ".tmp");
    out << "{truncated";
  }
  assert(load_config(primary).device_token == approved.device_token);
  assert(!fs::exists(primary + ".tmp"));

  // If both install and rollback fail, preserve both files for next-launch
  // recovery.
  fail_install = fail_restore = true;
  rejects([&] { save_config(primary, initial); });
  assert(!fs::exists(primary));
  assert(fs::exists(primary + ".tmp") && fs::exists(primary + ".bak"));
  fail_install = fail_restore = false;
  assert(load_config(primary).server_url == initial.server_url);
  assert(!load_config(primary).paired());

  // Standard Linux atomic-replace behavior remains supported.
  refuse_overwrite = false;
  save_config(primary, approved);
  assert(load_config(primary).paired());
  fs::remove_all(root);
  std::cout << "Configuration replacement/recovery tests passed (Switch rename "
               "semantics)\n";
}
