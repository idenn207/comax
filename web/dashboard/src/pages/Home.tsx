import { Card, Container, Flex, Heading, Text } from '@radix-ui/themes';

/**
 * Scaffold home page. Replaced by the projects list + bento overview in
 * Task 7. Today it exists only to prove the SPA shell boots and the
 * Radix Theme provider renders.
 */
export function HomePage() {
  return (
    <Container size="3" py="9">
      <Flex direction="column" gap="4">
        <Heading size="8">Comax Secrets</Heading>
        <Text color="gray" size="4">
          Dashboard scaffold — Task 7부터 프로젝트/환경/시크릿 화면이 들어옵니다.
        </Text>
        <Card variant="surface">
          <Flex direction="column" gap="2">
            <Text weight="medium">Foundation 상태</Text>
            <Text color="gray" size="2">
              SPA shell가 secret-server에서 정상적으로 서빙됩니다. CSP nonce도 같은 응답에 포함되어
              있습니다.
            </Text>
          </Flex>
        </Card>
      </Flex>
    </Container>
  );
}
