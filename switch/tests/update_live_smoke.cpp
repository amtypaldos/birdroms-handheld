#include "api/client.hpp"
#include "config/storage.hpp"
#include "update/updater.hpp"
#include <curl/curl.h>
#include <cassert>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <thread>
#include <unistd.h>
namespace fs=std::filesystem;
std::string read(const std::string& p){std::ifstream f(p,std::ios::binary);return {std::istreambuf_iterator<char>(f),{}};}
int main(int argc,char**argv){assert(argc==4);using namespace bird;curl_global_init(CURL_GLOBAL_DEFAULT);
 auto root=fs::temp_directory_path()/("birdroms-live-smoke-"+std::to_string(getpid()));fs::create_directories(root);
 auto target=(root/"birdroms.nro").string();fs::copy_file(argv[3],target);
 Config config;config.server_url=argv[1];config.device_token=read(argv[2]);config.rom_base=(root/"roms").string();config.platform_dirs["NES"]="my-nes";
 auto path=(root/"config.json").string();save_paired_config(path,config);auto before=read(path);
 {Updater updater(config,target,"2026.11.01.1");updater.check();while(updater.snapshot().running)std::this_thread::sleep_for(std::chrono::milliseconds(10));auto info=updater.snapshot();if(!info.available){std::cerr<<info.message<<"\n";return 1;}assert(info.info.version=="2026.11.02");
 updater.start();while(updater.snapshot().running)std::this_thread::sleep_for(std::chrono::milliseconds(10));auto staged=updater.snapshot();if(!staged.ready){std::cerr<<staged.message<<"\n";return 1;}assert(staged.written>0);}
 assert(read(path)==before);apply_staged_update(target);assert(read(path)==before&&fs::exists(update_backup_path(target)));
 auto saved=load_config(path);saved.rom_base=(root/"chosen").string();save_paired_config(path,saved);assert(load_config(path).device_token==config.device_token&&load_config(path).rom_base==saved.rom_base);
 fs::remove_all(root);curl_global_cleanup();std::cout<<"Released server image: device-authenticated check/download/install succeeded; JSON config preserved and writable\n";
}
