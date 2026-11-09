import * as AuthSession from 'expo-auth-session';
import * as WebBrowser from 'expo-web-browser';
import {
  createContext,
  PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { Platform } from 'react-native';

import { environment } from '@/config/environment';

import { loadToken, removeToken, saveToken } from './token-storage';

WebBrowser.maybeCompleteAuthSession();

type SessionToken = {
  accessToken: string;
  refreshToken?: string;
  idToken?: string;
  tokenType: string;
  expiresIn?: number;
  issuedAt: number;
};

type AuthContextValue = {
  accessToken: string | null;
  error: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: () => Promise<void>;
  register: () => Promise<void>;
  logout: () => Promise<void>;
  getValidAccessToken: () => Promise<string | null>;
};

const AuthContext = createContext<AuthContextValue | null>(null);
const scopes = ['openid', 'profile', 'email', 'offline_access'];

function makeRedirectUri() {
  if (Platform.OS === 'web') {
    return AuthSession.makeRedirectUri({ path: 'oauth/callback' });
  }

  return AuthSession.makeRedirectUri({ scheme: 'web3-user', path: 'oauth/callback' });
}

function toSessionToken(response: AuthSession.TokenResponse): SessionToken {
  return {
    accessToken: response.accessToken,
    refreshToken: response.refreshToken,
    idToken: response.idToken,
    tokenType: response.tokenType,
    expiresIn: response.expiresIn,
    issuedAt: response.issuedAt,
  };
}

function isFresh(token: SessionToken, marginSeconds = 60): boolean {
  if (!token.expiresIn) return true;
  return token.issuedAt + token.expiresIn - marginSeconds > Date.now() / 1000;
}

export function AuthProvider({ children }: PropsWithChildren) {
  const discovery = AuthSession.useAutoDiscovery(environment.keycloakIssuer);
  const redirectUri = useMemo(() => makeRedirectUri(), []);
  const [token, setToken] = useState<SessionToken | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const processedCode = useRef<string | null>(null);

  const [request, response, promptAsync] = AuthSession.useAuthRequest(
    {
      clientId: environment.keycloakClientId,
      redirectUri,
      responseType: AuthSession.ResponseType.Code,
      scopes,
      usePKCE: true,
      codeChallengeMethod: AuthSession.CodeChallengeMethod.S256,
    },
    discovery,
  );
  const responseError =
    response?.type === 'error'
      ? response.error?.message ?? 'Keycloak 登录失败'
      : null;

  const persistToken = useCallback(async (nextToken: SessionToken | null) => {
    setToken(nextToken);
    if (nextToken) {
      await saveToken(JSON.stringify(nextToken));
    } else {
      await removeToken();
    }
  }, []);

  useEffect(() => {
    let active = true;
    void loadToken()
      .then((stored) => {
        if (active && stored) setToken(JSON.parse(stored) as SessionToken);
      })
      .catch(() => {
        if (active) setError('读取本地登录状态失败');
      })
      .finally(() => {
        if (active) setIsLoading(false);
      });

    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (
      response?.type !== 'success' ||
      !discovery ||
      !request?.codeVerifier ||
      !response.params.code ||
      processedCode.current === response.params.code
    ) return;

    processedCode.current = response.params.code;
    setIsLoading(true);
    setError(null);
    void AuthSession.exchangeCodeAsync(
      {
        clientId: environment.keycloakClientId,
        code: response.params.code,
        redirectUri,
        extraParams: { code_verifier: request.codeVerifier },
      },
      discovery,
    )
      .then((result) => persistToken(toSessionToken(result)))
      .catch((cause: unknown) => {
        setError(cause instanceof Error ? cause.message : '授权码交换 Token 失败');
      })
      .finally(() => setIsLoading(false));
  }, [discovery, persistToken, redirectUri, request, response]);

  const login = useCallback(async () => {
    setError(null);
    if (!request || !discovery) {
      setError('Keycloak 配置仍在加载，请稍后重试');
      return;
    }
    await promptAsync();
  }, [discovery, promptAsync, request]);

  const register = useCallback(async () => {
    setError(null);
    if (!request?.url || !discovery) {
      setError('Keycloak 配置仍在加载，请稍后重试');
      return;
    }

    // Keycloak 26.7 推荐通过 OIDC prompt=create 直接进入注册流程。
    const registrationURL = new URL(request.url);
    registrationURL.searchParams.set('prompt', 'create');
    await promptAsync({ url: registrationURL.toString() });
  }, [discovery, promptAsync, request]);

  const refresh = useCallback(async (): Promise<SessionToken | null> => {
    if (!token?.refreshToken || !discovery) {
      await persistToken(null);
      return null;
    }
    try {
      const refreshed = await AuthSession.refreshAsync(
        { clientId: environment.keycloakClientId, refreshToken: token.refreshToken, scopes },
        discovery,
      );
      const nextToken = toSessionToken(refreshed);
      nextToken.refreshToken ??= token.refreshToken;
      await persistToken(nextToken);
      return nextToken;
    } catch (cause) {
      await persistToken(null);
      setError(cause instanceof Error ? cause.message : '刷新登录状态失败');
      return null;
    }
  }, [discovery, persistToken, token]);

  const getValidAccessToken = useCallback(async () => {
    if (!token) return null;
    if (isFresh(token)) return token.accessToken;
    return (await refresh())?.accessToken ?? null;
  }, [refresh, token]);

  const logout = useCallback(async () => {
    const idToken = token?.idToken;
    await persistToken(null);
    if (!discovery?.endSessionEndpoint) return;

    const postLogoutRedirectUri = AuthSession.makeRedirectUri({ scheme: 'web3-user' });
    const query = new URLSearchParams({
      client_id: environment.keycloakClientId,
      post_logout_redirect_uri: postLogoutRedirectUri,
    });
    if (idToken) query.set('id_token_hint', idToken);
    await WebBrowser.openAuthSessionAsync(
      `${discovery.endSessionEndpoint}?${query.toString()}`,
      postLogoutRedirectUri,
    );
  }, [discovery, persistToken, token]);

  const value = useMemo<AuthContextValue>(() => ({
    accessToken: token?.accessToken ?? null,
    error: error ?? responseError,
    isAuthenticated: Boolean(token?.accessToken),
    isLoading,
    login,
    register,
    logout,
    getValidAccessToken,
  }), [error, getValidAccessToken, isLoading, login, logout, register, responseError, token?.accessToken]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth 必须在 AuthProvider 内使用');
  return value;
}
