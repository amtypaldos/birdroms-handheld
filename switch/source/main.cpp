#include "config/storage.hpp"
#include "ui/app.hpp"
#include <cstdio>
#include <curl/curl.h>
#include <exception>
#include <switch.h>

namespace {
void startup_error() {
  consoleInit(nullptr);
  // Do not expose configuration contents or exception messages here.
  printf("birdROMs could not start.\nCheck config.json, SD card and available "
         "memory.\nPress + to exit.\n");
  PadState pad;
  padConfigureInput(1, HidNpadStyleSet_NpadStandard);
  padInitializeDefault(&pad);
  while (appletMainLoop()) {
    padUpdate(&pad);
    if (padGetButtonsDown(&pad) & HidNpadButton_Plus)
      break;
    consoleUpdate(nullptr);
  }
  consoleExit(nullptr);
}
} // namespace

int main(int, char **) {
  const bool romfs = R_SUCCEEDED(romfsInit());
  const bool sockets = R_SUCCEEDED(socketInitializeDefault());
  const bool ssl = sockets && R_SUCCEEDED(sslInitialize(4));
  const bool curl = ssl && curl_global_init(CURL_GLOBAL_DEFAULT) == CURLE_OK;
  int result = 1;
  if (romfs && curl) {
    try {
      const std::string path = "sdmc:/switch/birdroms/config.json";
      result = bird::run_ui(bird::load_config(path), path);
    } catch (const std::exception &) {
      result = 1;
    }
  }
  if (result)
    startup_error();
  if (curl)
    curl_global_cleanup();
  if (ssl)
    sslExit();
  if (sockets)
    socketExit();
  if (romfs)
    romfsExit();
  return result;
}
