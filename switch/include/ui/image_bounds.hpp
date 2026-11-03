#pragma once
#include <cstddef>
#include <cstdint>
#include <vector>

namespace bird {
// Check dimensions before SDL_image allocates decoded pixels. Only PNG and JPEG
// are accepted; other image formats retain the normal missing-cover
// placeholder.
inline bool cover_dimensions_safe(const std::vector<unsigned char> &data) {
  constexpr uint64_t max_pixels = 4 * 1024 * 1024;
  const auto allowed = [](uint32_t width, uint32_t height) {
    return width != 0 && height != 0 && width <= 4096 && height <= 4096 &&
           uint64_t(width) * height <= max_pixels;
  };
  const auto be16 = [&](size_t at) -> uint32_t {
    return (uint32_t(data[at]) << 8) | data[at + 1];
  };
  const auto be32 = [&](size_t at) -> uint32_t {
    return (uint32_t(data[at]) << 24) | (uint32_t(data[at + 1]) << 16) |
           (uint32_t(data[at + 2]) << 8) | data[at + 3];
  };
  const unsigned char png_signature[] = {137, 80, 78, 71, 13, 10, 26, 10};
  bool png = data.size() >= sizeof(png_signature);
  for (size_t i = 0; png && i < sizeof(png_signature); ++i)
    png = data[i] == png_signature[i];
  if (png) {
    // IHDR is required to be the first chunk, and contains 13 payload bytes
    // plus four CRC bytes. SDL_image validates the rest during decode.
    if (data.size() < 33 || be32(8) != 13 || data[12] != 'I' ||
        data[13] != 'H' || data[14] != 'D' || data[15] != 'R')
      return false;
    return allowed(be32(16), be32(20));
  }
  if (data.size() < 4 || data[0] != 0xff || data[1] != 0xd8)
    return false;
  size_t position = 2;
  bool have_dimensions = false;
  while (position < data.size()) {
    if (data[position++] != 0xff)
      return false;
    while (position < data.size() && data[position] == 0xff)
      ++position;
    if (position == data.size())
      return false;
    const unsigned marker = data[position++];
    if (marker == 0 || marker == 0xd8 || marker == 0xd9)
      return false;
    // Standalone markers are not expected before the entropy-coded scan.
    if (marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7))
      return false;
    if (data.size() - position < 2)
      return false;
    const size_t length = be16(position);
    if (length < 2 || length > data.size() - position)
      return false;
    if (marker == 0xda)
      return have_dimensions; // SOS: libjpeg has already received its SOF.
    const bool sof = marker >= 0xc0 && marker <= 0xcf && marker != 0xc4 &&
                     marker != 0xc8 && marker != 0xcc;
    if (sof) {
      if (have_dimensions || length < 8)
        return false;
      const unsigned components = data[position + 7];
      if (components == 0 || components > 4 || length != 8 + components * 3)
        return false;
      if (!allowed(be16(position + 5), be16(position + 3)))
        return false;
      have_dimensions = true;
    }
    position += length;
  }
  return false;
}
} // namespace bird
