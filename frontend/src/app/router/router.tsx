import { createBrowserRouter } from 'react-router';
import { AppLayout, RootIndexStub } from '@/app/layout/AppLayout';
import { RequireAuth } from '@/features/auth/RequireAuth';
import { FlagsPage } from '@/pages/FlagsPage';
import { LoginPage } from '@/pages/LoginPage';
import { NotFoundPage } from '@/pages/NotFoundPage';
import { StatusPage } from '@/pages/StatusPage';
import { UsersPage } from '@/pages/UsersPage';

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/status',
    element: <StatusPage />,
  },
  {
    path: '/',
    element: (
      <RequireAuth>
        <AppLayout />
      </RequireAuth>
    ),
    children: [
      {
        index: true,
        element: <RootIndexStub />,
      },
      {
        path: 'users',
        element: <UsersPage />,
      },
      {
        path: 'flags',
        element: <FlagsPage />,
      },
    ],
  },
  {
    path: '*',
    element: <NotFoundPage />,
  },
]);
