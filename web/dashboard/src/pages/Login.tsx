import { Card, Container, Flex, Heading, Text } from '@radix-ui/themes';

/**
 * Scaffold login page. Task 6 wires the actual flow: paste bearer →
 * POST /api/v1/dashboard/session → store the returned CSRF → redirect
 * to "/". The shell here just confirms routing works.
 */
export function LoginPage() {
  return (
    <Container size="2" py="9">
      <Flex direction="column" gap="4">
        <Heading size="6">로그인</Heading>
        <Card variant="surface">
          <Flex direction="column" gap="2">
            <Text weight="medium">Task 6에서 구현</Text>
            <Text color="gray" size="2">
              서비스 토큰을 붙여넣으면 dashboard 세션 쿠키 + CSRF 토큰을 발급받고 홈으로
              리다이렉트합니다.
            </Text>
          </Flex>
        </Card>
      </Flex>
    </Container>
  );
}
