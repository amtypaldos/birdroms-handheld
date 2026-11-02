#include "api/client.hpp"
#include "update/parsing.hpp"
#include "update/picosha2.h"
#include "update/updater.hpp"
#include <cassert>
#include <cerrno>
#include <chrono>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <nlohmann/json.hpp>
#include <unistd.h>
using namespace bird;
namespace fs = std::filesystem;
std::string root, fail_target;
bool slow = false;
std::string read(const std::string &p) {
  std::ifstream f(p, std::ios::binary);
  return std::string(std::istreambuf_iterator<char>(f), {});
}
extern "C" int __real_rename(const char *, const char *);
extern "C" int __wrap_rename(const char *from, const char *to) {
  if (!fail_target.empty() && std::string(from) == fail_target + ".update" &&
      std::string(to) == fail_target) {
    fail_target.clear();
    errno = EACCES;
    return -1;
  }
  return __real_rename(from, to);
}
namespace bird {
UpdateInfo Client::handheld_info(std::atomic_bool *cancel) {
  if (slow)
    while (!cancel->load())
      std::this_thread::sleep_for(std::chrono::milliseconds(5));
  if (cancel && cancel->load())
    throw std::runtime_error("cancelled");
  return parse_update_info(read(root + "/metadata.json"));
}
void Client::download_handheld(
    FILE *out, std::atomic_bool &cancel,
    const std::function<void(int64_t, int64_t)> &cb) {
  auto data = read(root + "/valid.zip");
  if (cancel)
    throw std::runtime_error("cancelled");
  assert(fwrite(data.data(), 1, data.size(), out) == data.size());
  cb(data.size(), data.size());
}
} // namespace bird
void await(Updater &u) {
  for (int i = 0; i < 1000 && u.snapshot().running; i++)
    std::this_thread::sleep_for(std::chrono::milliseconds(5));
  assert(!u.snapshot().running);
}
int main(int argc, char **argv) {
  assert(argc == 2);
  root = fs::absolute(argv[1]).string();
  const auto target = root + "/app.nro", config = read(root + "/config.json"),
             old = read(target), rom = read(root + "/Game.nes");
  auto info = parse_update_info(read(root + "/metadata.json"));
  std::atomic_bool cancel{false};
  assert(update_needed("2026.11.01.1", info.version));
  assert(!update_needed("2026.11.01.2", info.version));
  assert(!update_needed("2026.11.02.1", info.version));
  assert(update_needed("2026.11.01.9", "2026.11.01.10"));
  assert(!parse_update_info("{\"available\":false}").available);
  bool rejected = false;
  try {
    parse_update_info("{\"available\":true}");
  } catch (...) {
    rejected = true;
  }
  assert(rejected);
  assert(update_executable_path("app.nro").empty());
  assert(update_executable_path(root + "/../app.nro").empty());
  assert(update_executable_path(target) == target);
  for (const char *name :
       {"traversal.zip", "duplicate.zip", "config.zip", "seed-secret.zip",
        "missing.zip", "manifest.zip", "manifest-extra.zip",
        "manifest-duplicate.zip", "fake-dir.zip", "invalid-nro.zip",
        "symlink.zip", "oversize.zip"}) {
    rejected = false;
    try {
      stage_update_archive(root + "/" + name, target, info, cancel);
    } catch (...) {
      rejected = true;
    }
    assert(rejected);
    assert(read(target) == old);
    assert(!fs::exists(update_stage_path(target)));
    assert(!fs::exists(target + ".update.tmp"));
  }
  cancel = true;
  rejected = false;
  try {
    stage_update_archive(root + "/valid.zip", target, info, cancel);
  } catch (...) {
    rejected = true;
  }
  assert(rejected);
  cancel = false;
  auto incorrect = info;
  incorrect.nro_sha256 = std::string(64, '0');
  rejected = false;
  try {
    stage_update_archive(root + "/valid.zip", target, incorrect, cancel);
  } catch (...) {
    rejected = true;
  }
  assert(rejected);
  fs::create_symlink(config.empty() ? target : root + "/config.json",
                     target + ".update.tmp");
  rejected = false;
  try {
    stage_update_archive(root + "/valid.zip", target, info, cancel);
  } catch (...) {
    rejected = true;
  }
  assert(rejected);
  fs::remove(target + ".update.tmp");
  stage_update_archive(root + "/valid.zip", target, info, cancel);
  assert(read(target) == old);
  assert(fs::exists(target + ".update"));
  fail_target = target;
  rejected = false;
  try {
    apply_staged_update(target);
  } catch (...) {
    rejected = true;
  }
  assert(rejected && read(target) == old);
  assert(fs::exists(target + ".update"));
  apply_staged_update(target);
  assert(read(target) != old);
  assert(read(update_backup_path(target)) == old);
  assert(!fs::exists(target + ".update"));
  assert(read(root + "/config.json") == config &&
         read(root + "/Game.nes") == rom);
  // Config remains writable after installation; token and user settings
  // survive.
  {
    std::ofstream f(root + "/config.json", std::ios::app);
    f << "\n";
  }
  assert(read(root + "/config.json") == config + "\n");
  // Recheck the staged bytes at apply time, rejecting modifications after
  // verification.
  stage_update_archive(root + "/valid.zip", target, info, cancel);
  {
    std::fstream f(target + ".update",
                   std::ios::in | std::ios::out | std::ios::binary);
    f.seekp(0x90);
    f.put('!');
  }
  auto current = read(target);
  rejected = false;
  try {
    apply_staged_update(target);
  } catch (...) {
    rejected = true;
  }
  assert(rejected && read(target) == current);
  fs::remove(target + ".update");
  fs::remove(target + ".update.json");
  // Recover a power-loss gap from the launchable .previous.nro before applying.
  stage_update_archive(root + "/valid.zip", target, info, cancel);
  fs::remove(target);
  apply_staged_update(target);
  assert(fs::exists(target) && fs::exists(update_backup_path(target)));
  Config c;
  c.server_url = "https://example.invalid";
  c.device_token = "fixture";
  Updater u(c, target, "2026.11.01.1");
  u.check();
  await(u);
  assert(u.snapshot().available);
  u.start();
  await(u);
  assert(u.snapshot().ready);
  assert(read(root + "/config.json") == config + "\n");
  slow = true;
  Updater cancelled(c, target, "2026.11.01.1");
  cancelled.check();
  cancelled.cancel();
  await(cancelled);
  assert(!cancelled.snapshot().ready);
  assert(cancelled.snapshot().cancelled);
  cancelled.check();
  assert(!cancelled.snapshot().cancelled);
  cancelled.cancel();
  await(cancelled);
  slow = false;
  std::cout << "Update "
               "metadata/version/archive/traversal/symlink/hash/cancellation/"
               "rollback/config-preservation tests passed\n";
}
