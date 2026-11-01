#include "config/folders.hpp"
#include <cassert>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <unistd.h>
namespace fs = std::filesystem;
template <class F> void rejects(F fn) {
  bool rejected = false;
  try {
    fn();
  } catch (const std::exception &) {
    rejected = true;
  }
  assert(rejected);
}
int main() {
  using namespace bird;
  auto root = fs::temp_directory_path() /
              ("birdroms-folders-" + std::to_string(getpid()));
  fs::remove_all(root);
  fs::create_directories(root / "roms" / "NES");
  fs::create_directories(root / "empty");
  {
    std::ofstream f(root / "file.nro");
    f << "not a folder";
  }
  fs::create_directory_symlink(root / "roms", root / "linked");
  assert(folder_parent("sdmc:/", "sdmc:/retroarch/") == "sdmc:/");
  assert(folder_parent("sdmc:/", "sdmc:/") == "sdmc:/");
  assert(folder_child("sdmc:/", "sdmc:/", "ROMs") == "sdmc:/ROMs/");
  auto base = root.string() + "/";
  auto page = list_folders(base, base);
  assert((page.directories == std::vector<std::string>{"empty", "roms"}));
  assert(folder_start(base, (root / "roms" / "missing" / "child").string()) ==
         (root / "roms").string() + "/");
  assert(folder_start(base, (root / "unknown").string()) == base);
  assert(folder_parent(base, base) == base);
  assert(folder_parent(base, (root / "roms" / "NES").string()) ==
         (root / "roms").string() + "/");
  assert(folder_child(base, base, "roms") == root.string() + "/roms/");
  assert(list_folders(base, (root / "empty").string()).directories.empty());
  rejects([&] { folder_child(base, base, "../outside"); });
  rejects([&] { folder_child(base, base, "sdmc:/"); });
  rejects([&] { folder_child(base, base, "bad\\path"); });
  rejects([&] { list_folders(base, (root / "linked").string()); });
  rejects([&] { list_folders(base, (root / ".." / "escape").string()); });
  rejects([&] { list_folders(base, root.string() + "-other"); });
  rejects([&] { list_folders(base, (root / "file.nro").string()); });
  fs::remove_all(root);
  std::cout << "Directory picker tests passed: listing, navigation, missing "
               "paths, SD boundary and symlink safety\n";
}
