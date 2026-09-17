import { Button, Center, Container, Stack, Text, Title } from '@mantine/core';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';

export function NotFoundPage() {
  const { t } = useTranslation();

  return (
    <Center mih="100dvh" p="md">
      <Container size={420} w="100%">
        <Stack gap="md" align="center">
          <Title order={1} ta="center">
            404
          </Title>
          <Text ta="center" fw={600}>
            {t('notFound.title')}
          </Text>
          <Text ta="center" c="dimmed">
            {t('notFound.message')}
          </Text>
          <Button component={Link} to="/">
            {t('notFound.backHome')}
          </Button>
        </Stack>
      </Container>
    </Center>
  );
}
