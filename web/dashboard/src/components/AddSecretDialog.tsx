import { useState, type FormEvent } from 'react';
import { Button, Callout, Dialog, Flex, Text, TextArea, TextField } from '@radix-ui/themes';
import { useMutation, useQueryClient } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { putSecret, queryKeys } from '../lib/queries';
import { nameError } from '../lib/validate';
import { useToast } from './Toast';

interface AddSecretDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectName: string;
  envName: string;
  existingKeys: ReadonlySet<string>;
}

export function AddSecretDialog({
  open,
  onOpenChange,
  projectName,
  envName,
  existingKeys,
}: AddSecretDialogProps) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [key, setKey] = useState('');
  const [value, setValue] = useState('');
  const [error, setError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: ({ k, v }: { k: string; v: string }) => putSecret(projectName, envName, k, v),
    onSuccess: async (secret) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.secrets(projectName, envName) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.versions(projectName, envName) }),
      ]);
      toast.notify('success', `시크릿 "${secret.key}" 저장됨 (v${secret.version})`);
      setKey('');
      setValue('');
      onOpenChange(false);
    },
    onError: (err: unknown) => {
      setError(err instanceof ApiError ? err.message : '저장에 실패했습니다.');
    },
  });

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    const trimmedKey = key.trim();
    const validation = nameError('key', trimmedKey);
    if (validation) {
      setError(validation);
      return;
    }
    if (existingKeys.has(trimmedKey)) {
      setError('같은 이름의 키가 이미 존재합니다. 표에서 직접 편집해 주세요.');
      return;
    }
    mutation.mutate({ k: trimmedKey, v: value });
  }

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setKey('');
          setValue('');
          setError(null);
          mutation.reset();
        }
        onOpenChange(next);
      }}
    >
      <Dialog.Content maxWidth="520px">
        <Dialog.Title>새 시크릿</Dialog.Title>
        <Dialog.Description size="2" mb="3">
          저장 즉시 새 버전이 생성되며 감사 로그에 기록됩니다.
        </Dialog.Description>
        <form onSubmit={onSubmit}>
          <Flex direction="column" gap="3">
            <label>
              <Text as="div" size="2" mb="1" weight="medium">
                키
              </Text>
              <TextField.Root
                value={key}
                onChange={(e) => setKey(e.target.value)}
                placeholder="예: DATABASE_URL"
                autoFocus
                spellCheck={false}
                aria-label="키 이름"
              />
            </label>
            <label>
              <Text as="div" size="2" mb="1" weight="medium">
                값
              </Text>
              <TextArea
                value={value}
                onChange={(e) => setValue(e.target.value)}
                placeholder="값을 입력하세요"
                rows={4}
                spellCheck={false}
                aria-label="시크릿 값"
              />
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
              <Button type="submit" disabled={mutation.isPending || key.trim() === ''}>
                {mutation.isPending ? '저장 중…' : '저장'}
              </Button>
            </Flex>
          </Flex>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  );
}
