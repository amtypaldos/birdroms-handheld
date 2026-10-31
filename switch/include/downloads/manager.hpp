#pragma once
#include "api/client.hpp"
#include <mutex>
#include <thread>
namespace bird {
struct DownloadState {
  bool running = false, finished = false, unauthorized = false;
  size_t index = 0, count = 0, done = 0, skipped = 0, failed = 0;
  int64_t written = 0, total = 0;
  double speed = 0;
  std::string title, message, last_error;
};
class Downloads {
  std::thread worker_;
  std::atomic_bool cancel_{false};
  mutable std::mutex mutex_;
  DownloadState state_;

public:
  ~Downloads();
  void start(Config, std::vector<Rom>);
  void cancel();
  DownloadState snapshot() const;
};
} // namespace bird
