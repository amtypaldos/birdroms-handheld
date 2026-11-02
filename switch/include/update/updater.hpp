#pragma once
#include "types.hpp"
#include <atomic>
#include <mutex>
#include <thread>
namespace bird {
struct UpdateInfo {
  bool available = false;
  std::string version, filename, sha256, nro_sha256;
  int64_t size = 0, nro_size = 0;
};
struct UpdateState {
  bool running = false, checked = false, available = false, ready = false,
       cancelled = false;
  UpdateInfo info;
  int64_t written = 0, total = 0;
  double speed = 0;
  std::string message;
};
class Updater {
  Config config_;
  std::string executable_, version_;
  std::thread worker_;
  std::atomic_bool cancel_{false};
  mutable std::mutex mutex_;
  UpdateState state_;

public:
  Updater(Config, std::string executable, std::string current_version);
  ~Updater();
  void check();
  void start();
  void cancel();
  UpdateState snapshot() const;
};
void stage_update_archive(const std::string &archive,
                          const std::string &executable, const UpdateInfo &,
                          std::atomic_bool &cancel);
std::string update_stage_path(const std::string &executable);
std::string update_backup_path(const std::string &executable);
bool update_needed(const std::string &installed, const std::string &offered);
// Resolve only the actual SD executable passed by Homebrew Menu; no guessed
// fallback.
std::string update_executable_path(const std::string &argv0);
// Called by main only AFTER all SDL/ROMFS/network resources have closed.
void apply_staged_update(const std::string &executable);
} // namespace bird
