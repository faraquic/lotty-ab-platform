import { useMutation } from '@tanstack/react-query';
import type { UseMutationResult } from '@tanstack/react-query';
import { loginRequest } from './login';
import type { LoginApiError, LoginData, LoginRequest } from '../types';

export function useLoginMutation(): UseMutationResult<LoginData, LoginApiError, LoginRequest> {
  return useMutation<LoginData, LoginApiError, LoginRequest>({
    mutationFn: loginRequest,
  });
}
