#include "api/client.hpp"
#include "config/storage.hpp"
#include "update/parsing.hpp"
#include "update/updater.hpp"
#include <cassert>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <unistd.h>
namespace fs=std::filesystem;
std::string read(const std::string& path){std::ifstream in(path,std::ios::binary);return {std::istreambuf_iterator<char>(in),{}};}
int main(int argc,char**argv){
 assert(argc==4);using namespace bird;
 auto info=parse_update_info(read(argv[2]));assert(info.available);
 auto root=fs::temp_directory_path()/("birdroms-real-update-"+std::to_string(getpid()));fs::create_directories(root);
 auto target=(root/"birdroms.nro").string();fs::copy_file(argv[3],target);
 Config config;config.server_url="https://example.test";config.device_token="fixture-token-not-in-package";config.rom_base=(root/"my-roms").string();config.platform_dirs["NES"]="custom-nes";
 auto path=(root/"config.json").string();save_paired_config(path,config);
 auto original=read(path);std::atomic_bool cancel{false};
 stage_update_archive(argv[1],target,info,cancel);assert(read(path)==original);
 apply_staged_update(target);assert(read(path)==original);assert(fs::exists(update_backup_path(target)));
 auto saved=load_config(path);assert(saved.device_token==config.device_token&&saved.rom_base==config.rom_base&&saved.platform_dirs==config.platform_dirs);
 saved.rom_base=(root/"new-roms").string();save_paired_config(path,saved);
 auto changed=load_config(path);assert(changed.rom_base==saved.rom_base&&changed.device_token==config.device_token);
 fs::remove_all(root);std::cout<<"Real personalized Switch ZIP accepted; runtime JSON remains writable and pairing/settings preserved\n";
}
