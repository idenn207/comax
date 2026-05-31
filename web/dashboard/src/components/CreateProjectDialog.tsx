import { useState, type FormEvent } from 'react';
import { Button, Callout, Dialog, Flex, Text, TextField } from '@radix-ui/themes';
import { useMutation, useQueryClient } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { createProject, queryKeys } from '../lib/queries';
import { nameError } from '../lib/validate';
import { useToast } from './Toast';

interface CreateProjectDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateProjectDialog({ open, onOpenChange }: CreateProjectDialogProps) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [name, setName] = useState('');
  const [error, setError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: (value: string) => createProject(value),
    onSuccess: async (project) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.projects() });
      toast.notify('success', `프로젝트 "${project.name}" 생성됨`);
      setName('');
      onOpenChange(false);
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        if (err.code === 'conflict') {
          setError('같은 이름의 프로젝트가 이미 존재합니다.');
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
    const validation = nameError('project name', trimmed);
    if (validation) {
      setError(validation);
      return;
    }
    mutation.mutate(trimmed);
  }

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setName('');
          setError(null);
          mutation.reset();
        }
        onOpenChange(next);
      }}
    >
      <Dialog.Content maxWidth="450px">
        <Dialog.Title>새 프로젝트</Dialog.Title>
        <Dialog.Description size="2" mb="3">
          프로젝트는 환경(local/staging/prod 등)과 시크릿을 묶는 최상위 단위입니다.
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
                placeholder="예: my-app"
                autoFocus
                spellCheck={false}
                aria-label="프로젝트 이름"
                aria-invalid={error !== null}
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
