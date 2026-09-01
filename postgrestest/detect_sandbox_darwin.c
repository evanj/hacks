#include "detect_sandbox_darwin.h"

#include <stdio.h>
#include <unistd.h>

// courtesy of clang
// https://github.com/applesrc/clang/blob/bb8f644/src/projects/compiler-rt/lib/sanitizer_common/sanitizer_mac_spi.cc

enum sandbox_filter_type {
  SANDBOX_FILTER_NONE,
  SANDBOX_FILTER_PATH,
  SANDBOX_FILTER_GLOBAL_NAME,
  SANDBOX_FILTER_LOCAL_NAME,
  SANDBOX_FILTER_APPLEEVENT_DESTINATION,
  SANDBOX_FILTER_RIGHT_NAME,
  SANDBOX_FILTER_PREFERENCE_DOMAIN,
  SANDBOX_FILTER_KEXT_BUNDLE_ID,
  SANDBOX_FILTER_INFO_TYPE,
  SANDBOX_FILTER_NOTIFICATION,
  SANDBOX_FILTER_FILE_DESCRIPTOR = 10,         // opcode: 240
  SANDBOX_FILTER_AUDIT_TOKEN_ATTR = 11,        // opcode: 241
  SANDBOX_FILTER_XPC_SERVICE_NAME = 12,        // opcode: 50
  SANDBOX_FILTER_IOKIT_CONNECTION = 13,        // opcode: 19
  SANDBOX_FILTER_IOKIT_USER_CLIENT_CLASS = 14, // opcode: 65
  SANDBOX_FILTER_NVRAM_VARIABLE = 15,          // opcode: 75
  SANDBOX_FILTER_SYSCTL_NAME = 16,             // opcode: 45
  SANDBOX_FILTER_POSIX_IPC_NAME = 17,          // opcode: 5
  SANDBOX_FILTER_MESSAGE_FILTER = 18,          // opcode: 52
};

typedef struct sandbox_profile {
  char *builtin;
  unsigned char *data;
  size_t size;
} *sandbox_profile_t;

typedef struct sandbox_params {
  const char **params;
  size_t size;
  size_t available;
} *sandbox_params_t;

int sandbox_check(pid_t pid, const char *operation,
                  enum sandbox_filter_type type, ...);

bool detect_mac_sandbox() {
  pid_t self_pid = getpid();
  // "ipc-sysv-shm" is the Mac sandbox-exec rule that allows shmget
  static const char SHM_OPERATION[] = "ipc-sysv-shm";
  printf("checking for %s with sandbox_check ...\n", SHM_OPERATION);
  int result = sandbox_check(self_pid, SHM_OPERATION, SANDBOX_FILTER_NONE);
  if (result == 0) {
    // 0 = no sandbox detected (ipc-sysv-shm is allowed)
    return false;
  } else if (result == 1) {
    // 1 = sandbox detected (ipc-sysv-shm is denied)
    return true;
  } else {
    fprintf(stderr,
            "sandbox_check returned an unexpected value result=%d (expected 0 "
            "or 1)\n",
            result);
    return false;
  }
}
