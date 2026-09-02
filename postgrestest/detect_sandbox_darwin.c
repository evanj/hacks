#include "detect_sandbox_darwin.h"

#include <stdio.h>
#include <unistd.h>

// sandbox_check is a private API reverse-engineered here:
// https://github.com/theos/headers/blob/master/sandbox.h
// https://github.com/Karmaz95/sb_validator/blob/main/src/main.c

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

int sandbox_check(pid_t pid, const char *operation, enum sandbox_filter_type type, ...);

bool detect_mac_sandbox() {
  pid_t self_pid = getpid();
  // "ipc-sysv-shm" is the Mac sandbox-exec rule that allows shmget
  static const char SHM_OPERATION[] = "ipc-sysv-shm";
  int result = sandbox_check(self_pid, SHM_OPERATION, SANDBOX_FILTER_NONE);
  if (result == 0) {
    // 0 = no sandbox detected (ipc-sysv-shm is allowed)
    return false;
  } else if (result == 1) {
    // 1 = sandbox detected (ipc-sysv-shm is denied)
    return true;
  } else {
    fprintf(
        stderr,
        "warning: sandbox_check returned an unexpected value result=%d (can't detect sandbox)\n",
        result);
    return false;
  }
}
