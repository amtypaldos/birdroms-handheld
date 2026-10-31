#include "downloads/manager.hpp"
#include "config/storage.hpp"
#include <filesystem>
#include <chrono>
#include <cstdio>
#include <cstring>
#include <cerrno>
#include <fcntl.h>
#include <unistd.h>
namespace bird {
Downloads::~Downloads(){cancel();if(worker_.joinable())worker_.join();}
void Downloads::cancel(){cancel_.store(true);}
DownloadState Downloads::snapshot()const{std::lock_guard<std::mutex> lock(mutex_);return state_;}
void Downloads::start(Config config,std::vector<Rom> roms){
 if(snapshot().running)throw std::runtime_error("Download queue is already running");
 if(worker_.joinable())worker_.join();
 cancel_=false;
 {std::lock_guard<std::mutex> lock(mutex_);state_=DownloadState{};state_.running=true;state_.count=roms.size();}
 worker_=std::thread([this,config=std::move(config),roms=std::move(roms)]{
  Client client(config);
  for(size_t i=0;i<roms.size()&&!cancel_;++i){
   const auto&r=roms[i];FILE* file=nullptr;
   {std::lock_guard<std::mutex>lock(mutex_);state_.index=i+1;state_.title=r.title;state_.written=0;state_.total=r.size_bytes;state_.speed=0;state_.message="Preparing";}
   try{
    if(existing(config,r)){std::lock_guard<std::mutex>lock(mutex_);++state_.done;++state_.skipped;state_.message="Already on SD card";continue;}
    std::string final=destination(config,r),part=final+".part";std::filesystem::create_directories(std::filesystem::path(final).parent_path());
    // Revalidate after directory creation, and refuse collisions at both clean and id names.
    if(destination(config,r)!=final)throw std::runtime_error("ROM path changed during preparation");
    std::error_code ec;if(std::filesystem::exists(std::filesystem::symlink_status(final,ec)))throw std::runtime_error("Destination filename is occupied by another file");
    if(std::filesystem::is_symlink(std::filesystem::symlink_status(part,ec)))throw std::runtime_error("Partial file is a symbolic link");
    auto available=std::filesystem::space(std::filesystem::path(final).parent_path(),ec);if(!ec&&r.size_bytes>0&&available.available<(uint64_t)r.size_bytes)throw std::runtime_error("Insufficient SD card storage");
    int flags=O_CREAT|O_TRUNC|O_WRONLY;
#ifdef O_NOFOLLOW
    flags|=O_NOFOLLOW;
#endif
    int fd=open(part.c_str(),flags,0644);if(fd<0)throw std::runtime_error(std::string("Cannot open partial file: ")+std::strerror(errno));
    file=fdopen(fd,"wb");if(!file){close(fd);throw std::runtime_error("Cannot open partial file stream");}
    auto began=std::chrono::steady_clock::now();
    client.download(r.id,file,cancel_,[this,began](int64_t written,int64_t total){double seconds=std::chrono::duration<double>(std::chrono::steady_clock::now()-began).count();std::lock_guard<std::mutex>lock(mutex_);state_.written=written;if(total>0)state_.total=total;state_.speed=seconds>0?written/seconds:0;state_.message="Downloading";});
    if(cancel_)throw std::runtime_error("Download cancelled");
    if(fflush(file)!=0||ferror(file))throw std::runtime_error(std::string("SD write failed: ")+std::strerror(errno));
    auto length=ftell(file);if(length<0)throw std::runtime_error("Cannot determine downloaded size");
    if(r.size_bytes>0&&length!=r.size_bytes)throw std::runtime_error("Incomplete ROM transfer; partial file retained");
    if(fsync(fileno(file))!=0)throw std::runtime_error(std::string("SD sync failed: ")+std::strerror(errno));
    int closed=fclose(file);file=nullptr;if(closed!=0)throw std::runtime_error("SD close failed");
    if(std::filesystem::exists(final))throw std::runtime_error("Destination appeared during download; partial retained");
    if(std::rename(part.c_str(),final.c_str())!=0)throw std::runtime_error(std::string("Cannot finalize ROM: ")+std::strerror(errno));
    {std::lock_guard<std::mutex>lock(mutex_);++state_.done;state_.message="Downloaded";}
   }catch(const std::exception&e){if(file)fclose(file);std::lock_guard<std::mutex>lock(mutex_);if(!cancel_)++state_.failed;state_.message=cancel_?"Cancelled; partial file retained":e.what();}
  }
  std::lock_guard<std::mutex>lock(mutex_);state_.running=false;state_.finished=true;if(cancel_)state_.message="Cancelled; partial files restart next time";else if(!state_.failed)state_.message="Downloads complete";
 });
}
}
