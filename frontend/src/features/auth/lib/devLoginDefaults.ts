import type { LoginFormValues } from '../LoginForm';

const LOCAL_DEV_EMAIL = 'root@labp.net';
const LOCAL_DEV_PASSWORD = 'root!@#$';

function readEnvString(name: 'VITE_DEV_LOGIN_EMAIL' | 'VITE_DEV_LOGIN_PASSWORD'): string | null {
  const value: unknown = import.meta.env[name];
  return typeof value === 'string' && value.length > 0 ? value : null;
}

export function getDevLoginDefaults(): LoginFormValues | null {
  if (!import.meta.env.DEV) {
    return null;
  }
  return {
    email: readEnvString('VITE_DEV_LOGIN_EMAIL') ?? LOCAL_DEV_EMAIL,
    password: readEnvString('VITE_DEV_LOGIN_PASSWORD') ?? LOCAL_DEV_PASSWORD,
  };
}
