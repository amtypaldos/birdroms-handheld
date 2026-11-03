#pragma once
#include "update/updater.hpp"
namespace bird {
enum class UpdateAction { None, Download, Install };
struct UpdatePolicy {
  bool skipped = false;
  void skip() { skipped = true; }
  void retry() { skipped = false; }
  UpdateAction next(const UpdateState &state, bool may_interrupt) const {
    if (skipped || !may_interrupt || state.running || state.cancelled)
      return UpdateAction::None;
    if (state.ready)
      return UpdateAction::Install;
    return state.checked && state.available ? UpdateAction::Download
                                            : UpdateAction::None;
  }
};
} // namespace bird
