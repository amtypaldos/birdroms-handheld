#include "config/storage.hpp"
#include "ui/app.hpp"
#include "update/updater.hpp"
#include <cstdio>
#include <curl/curl.h>
#include <exception>
#include <filesystem>
#include <switch.h>

namespace {
void notice(const char *message) {
  consoleInit(nullptr);
  // Controlled messages only: never expose config contents or network bodies.
  printf("%s\nPress + to exit.\n", message);
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

int main(int argc, char **argv) {
  const std::string executable =
      bird::update_executable_path(argc > 0 && argv && argv[0] ? argv[0] : "");
  const bool romfs = R_SUCCEEDED(romfsInit());
  const bool sockets = R_SUCCEEDED(socketInitializeDefault());
  const bool ssl = sockets && R_SUCCEEDED(sslInitialize(4));
  const bool curl = ssl && curl_global_init(CURL_GLOBAL_DEFAULT) == CURLE_OK;
  int result = 1;
  if (romfs && curl) {
    try {
      const std::string path = "sdmc:/switch/birdroms/config.json";
      const std::string seed =
          executable.empty()
              ? "sdmc:/switch/birdroms/server.json"
              : (std::filesystem::path(executable).parent_path() /
                 "server.json")
                    .string();
      result =
          bird::run_ui(bird::with_seed_server(bird::load_config(path), seed),
                       path, executable);
    } catch (const std::exception &) {
      result = 1;
    }
  }
  const bool install = result == bird::UpdateReadyExit;
  if (result && !install)
    notice("birdROMs could not start. Check config.json, SD card and available "
           "memory.");
  // The running NRO's RomFS may hold the executable open. No replacement before
  // UI/fonts/workers, curl, services and RomFS have all released their
  // resources.
  if (curl)
    curl_global_cleanup();
  if (ssl)
    sslExit();
  if (sockets)
    socketExit();
  if (romfs)
    romfsExit();
  if (install) {
    try {
      bird::apply_staged_update(executable);
      if (envHasNextLoad()) {
        const auto arguments = "\"" + executable + "\"";
        if (R_SUCCEEDED(envSetNextLoad(executable.c_str(), arguments.c_str())))
          return 0;
      }
      notice("Update installed. Open birdROMs again from Homebrew Menu.");
      return 0;
    } catch (const std::exception &) {
      notice("Update could not be installed. Reopen birdROMs or its "
             ".previous.nro backup.");
      return 1;
    }
  }
  return result;
}
