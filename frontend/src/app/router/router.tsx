import { createBrowserRouter } from 'react-router';
import { AppLayout, RootIndexStub } from '@/app/layout/AppLayout';
import { RequireAuth } from '@/features/auth/RequireAuth';
import { ExperimentDetailsPage } from '@/pages/ExperimentDetailsPage';
import { ExperimentsPage } from '@/pages/ExperimentsPage';
import { FlagsPage } from '@/pages/FlagsPage';
import { LoginPage } from '@/pages/LoginPage';
import { MetricsPage } from '@/pages/MetricsPage';
import { NotFoundPage } from '@/pages/NotFoundPage';
import { ReviewsPage } from '@/pages/ReviewsPage';
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
      {
        path: 'experiments',
        element: <ExperimentsPage />,
      },
      {
        path: 'experiments/:id',
        element: <ExperimentDetailsPage />,
      },
      {
        path: 'reviews',
        element: <ReviewsPage />,
      },
      {
        path: 'metrics',
        element: <MetricsPage />,
      },
    ],
  },
  {
    path: '*',
    element: <NotFoundPage />,
  },
]);
