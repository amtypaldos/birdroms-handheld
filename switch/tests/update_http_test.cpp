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
    c.device_token = "update-fixture-secret";
    Client api(c);
    auto i = api.handheld_info();
    assert(i.available && i.version == "2026.11.01.2" && i.nro_size == 256);
    std::atomic_bool cancel{false};
    FILE *f = tmpfile();
    assert(f);
    int64_t bytes = 0;
    api.download_handheld(f, cancel, [&](int64_t n, int64_t) { bytes = n; });
    assert(bytes > 0 && ftell(f) == bytes);
    fclose(f);
    c.server_url = std::string(argv[1]) + "/error";
    f = tmpfile();
    bool rejected = false;
    try {
      Client(c).download_handheld(f, cancel, {});
    } catch (const HttpError &e) {
      rejected = e.status == 500;
    }
    assert(rejected && ftell(f) == 0);
    fclose(f);
    c.server_url = std::string(argv[1]) + "/large";
    f = tmpfile();
    rejected = false;
    try {
      Client(c).download_handheld(f, cancel, {});
    } catch (const std::exception &) {
      rejected = true;
    }
    assert(rejected && ftell(f) <= 64LL * 1024 * 1024);
    fclose(f);
    c.server_url = std::string(argv[1]) + "/slow";
    std::thread stop([&] {
      std::this_thread::sleep_for(std::chrono::milliseconds(50));
      cancel = true;
    });
    rejected = false;
    try {
      Client(c).handheld_info(&cancel);
    } catch (const std::exception &) {
      rejected = true;
    }
    stop.join();
    assert(rejected);
  }
  curl_global_cleanup();
  std::cout << "Update real HTTP endpoint/auth/stream cap/failure/cancellation "
               "tests passed\n";
}
