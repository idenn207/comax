import { useState, type FormEvent } from 'react';
import { Button, Callout, Dialog, Flex, Select, Text, TextField } from '@radix-ui/themes';
import { useMutation, useQueryClient } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { createEnv, queryKeys } from '../lib/queries';
import type { Environment } from '../lib/types';
import { nameError } from '../lib/validate';
import { useToast } from './Toast';

interface CreateEnvDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectName: string;
  existingEnvs: Environment[];
}

// Sentinel option for "no inheritance" — Radix Select chokes on an empty
// string value, so we use a stable literal that is invalid as an env
// name (validateName forbids the space).
const NONE_VALUE = '(none)';

export function CreateEnvDialog({
  open,
  onOpenChange,
  projectName,
  existingEnvs,
}: CreateEnvDialogProps) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [name, setName] = useState('');
  const [inheritsFrom, setInheritsFrom] = useState<string>(NONE_VALUE);
  const [error, setError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: ({ envName, inherits }: { envName: string; inherits: string }) =>
      createEnv(projectName, envName, inherits),
    onSuccess: async (env) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.envs(projectName) });
      toast.notify('success', `환경 "${env.name}" 생성됨`);
      setName('');
      setInheritsFrom(NONE_VALUE);
      onOpenChange(false);
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        if (err.code === 'conflict') {
          setError('같은 이름의 환경이 이미 존재합니다.');
          return;
        }
        if (err.code === 'not_found') {
          setError('상속하려는 환경을 찾을 수 없습니다.');
          return;
        }
        setError(err.message);
        return;
      }
      setError('알 수 없는 오류로 생성에 실패했습니다.');
    },
  });

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    const trimmed = name.trim();
    const validation = nameError('env name', trimmed);
    if (validation) {
      setError(validation);
      return;
    }
    mutation.mutate({
      envName: trimmed,
      inherits: inheritsFrom === NONE_VALUE ? '' : inheritsFrom,
    });
  }

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setName('');
          setInheritsFrom(NONE_VALUE);
          setError(null);
          mutation.reset();
        }
        onOpenChange(next);
      }}
    >
      <Dialog.Content maxWidth="500px">
        <Dialog.Title>새 환경</Dialog.Title>
        <Dialog.Description size="2" mb="3">
          환경을 다른 환경에서 상속받으면 키 단위로 덮어쓰지 않은 값을 자동으로 물려받습니다 (예:
          prod ← base).
        </Dialog.Description>
        <form onSubmit={onSubmit}>
          <Flex direction="column" gap="3">
            <label>
              <Text as="div" size="2" mb="1" weight="medium">
                이름
              </Text>
              <TextField.Root
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="예: prod"
                autoFocus
                spellCheck={false}
                aria-label="환경 이름"
                aria-invalid={error !== null}
              />
            </label>
            <label>
              <Text as="div" size="2" mb="1" weight="medium">
                상속받을 환경 (선택)
              </Text>
              <Select.Root value={inheritsFrom} onValueChange={setInheritsFrom}>
                <Select.Trigger aria-label="상속받을 환경" />
                <Select.Content>
                  <Select.Item value={NONE_VALUE}>(상속 없음)</Select.Item>
                  {existingEnvs.map((env) => (
                    <Select.Item key={env.id} value={env.name}>
                      {env.name}
                    </Select.Item>
                  ))}
                </Select.Content>
              </Select.Root>
            </label>
            {error ? (
              <Callout.Root color="red" role="alert">
                <Callout.Text>{error}</Callout.Text>
              </Callout.Root>
            ) : null}
            <Flex gap="3" mt="2" justify="end">
              <Dialog.Close>
                <Button variant="soft" color="gray" type="button">
                  취소
                </Button>
              </Dialog.Close>
              <Button type="submit" disabled={mutation.isPending || name.trim() === ''}>
                {mutation.isPending ? '생성 중…' : '생성'}
              </Button>
            </Flex>
          </Flex>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  );
}
