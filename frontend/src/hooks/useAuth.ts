
import { computed, onMounted, ref } from 'vue';
import { clearSession, getStoredSession, getToken, saveSession } from '../api/client';
import { login } from '../api/auth';
import type { UserSession } from '../types/domain';

const session = ref<UserSession | null>(getStoredSession());
const loading = ref(false);
let authentication: Promise<UserSession> | null = null;

export async function ensureSession(force = false): Promise<UserSession> {
  const stored = getStoredSession();
  if (!force && stored) {
    session.value = stored;
    return stored;
  }
  if (!authentication) {
    loading.value = true;
    authentication = login().then((next) => {
      saveSession(next);
      session.value = next;
      return next;
    }).finally(() => {
      loading.value = false;
      authentication = null;
    });
  }
  return authentication;
}

export function useAuth() {
  onMounted(() => {
    if (!session.value) void ensureSession();
  });
  return {
    session,
    loading,
    authenticated: computed(() => Boolean(getToken())),
    logout: () => {
      clearSession();
      session.value = null;
      void ensureSession(true);
    },
  };
}
