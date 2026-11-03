#include "api/client.hpp"
#include <cassert>
#include <chrono>
#include <curl/curl.h>
#include <iostream>
#include <thread>
using namespace bird;
int main(int argc, char **argv) {
  assert(argc == 2);
  curl_global_init(CURL_GLOBAL_DEFAULT);
  {
    Config c;
    c.server_url = argv[1];
    c.device_token = "fixture-secret";
    Client api(c);
    assert(api.platforms() == std::vector<std::string>{"NES"});
    assert(api.playlists()[0].id == 7);
    assert(api.playlist(7)[0].id == 1);
    assert(api.roms("NES & SNES", 1).count == 1);
    auto s = api.pair_start();
    assert(s.code == "ABC123");
    assert(api.pair_poll("pending").status == PollStatus::Pending);
    assert(api.pair_poll("expired").status == PollStatus::Expired);
    assert(api.pair_poll("limited").status == PollStatus::RateLimited);
    assert(api.pair_poll("approved").token == "approved-secret");
    bool bad = false;
    try {
      api.pair_poll("missing");
    } catch (const HttpError &e) {
      bad = e.status == 404;
    }
    assert(bad);
    auto external = c.server_url;
    external.replace(external.find("127.0.0.1"), 9, "localhost");
    assert(api.cover(external + "/external").size() == 3);
    bad = false;
    try {
      api.cover(c.server_url + "/redirect");
    } catch (const HttpError &e) {
      bad = e.status == 302;
    }
    assert(bad);
    bad = false;
    try {
      api.cover(c.server_url + "/oversize");
    } catch (const std::exception &) {
      bad = true;
    }
    assert(bad);
    std::atomic_bool cancel{false};
    FILE *f = tmpfile();
    assert(f);
    int64_t bytes = 0;
    api.download(1, f, cancel, [&](int64_t n, int64_t) { bytes = n; });
    assert(ftell(f) == 65536 && bytes == 65536);
    fclose(f);
    f = tmpfile();
    bad = false;
    try {
      api.download(2, f, cancel, {});
    } catch (const HttpError &e) {
      bad = e.status == 500;
    }
    assert(bad && ftell(f) == 0);
    fclose(f);
    f = tmpfile();
    std::thread stop([&] {
      std::this_thread::sleep_for(std::chrono::milliseconds(100));
      cancel = true;
    });
    bad = false;
    try {
      api.download(3, f, cancel, {});
    } catch (const std::exception &) {
      bad = true;
    }
    stop.join();
    assert(bad);
    fclose(f);
    c.device_token = "bad\r\nInjected: true";
    bad = false;
    try {
      Client(c).platforms();
    } catch (const std::exception &) {
      bad = true;
    }
    assert(bad);
  }
  curl_global_cleanup();
  std::cout << "API HTTP/auth/streaming/cancellation tests passed\n";
}
