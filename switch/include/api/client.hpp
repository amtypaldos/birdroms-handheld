#pragma once
#include "types.hpp"
#include "update/updater.hpp"
#include <cstdio>
#include <stdexcept>
namespace bird {
struct HttpError : std::runtime_error {
  int status;
  HttpError(int s, const std::string &m) : std::runtime_error(m), status(s) {}
};
class Client {
  Config config_;

public:
  explicit Client(Config c) : config_(std::move(c)) {}
  UpdateInfo handheld_info(std::atomic_bool *cancel = nullptr);
  void download_handheld(FILE *, std::atomic_bool &,
                         const std::function<void(int64_t, int64_t)> &);
  std::vector<std::string> platforms();
  std::vector<Playlist> playlists();
  Page roms(const std::string &, int);
  std::vector<Rom> playlist(int);
  PairSession pair_start();
  PollResult pair_poll(const std::string &);
  std::vector<unsigned char> cover(const std::string &);
  void download(int, FILE *, std::atomic_bool &,
                const std::function<void(int64_t, int64_t)> &);
};
} // namespace bird
