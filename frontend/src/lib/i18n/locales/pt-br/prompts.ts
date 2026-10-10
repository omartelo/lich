import type { Shape } from "../../catalog"
import type { prompts as en } from "../en/prompts"

export const prompts = {
  pullRequest: {
    conflicts: "O pull request #{number} ({branch}) tem conflitos de merge com {base}. Resolva-os.",
    checksUnnamed: {
      one: "O CI está falhando no pull request #{number} ({branch}). {count} check está vermelho; descubra qual e corrija.",
      other:
        "O CI está falhando no pull request #{number} ({branch}). {count} checks estão vermelhos; descubra quais e corrija.",
    },
    checksListed:
      "O CI está falhando no pull request #{number} ({branch}). Corrija estes checks:\n\n{checks}",
    checksOmitted: {
      one: "\n\n…e mais {count} check com falha não listado.",
      other: "\n\n…e mais {count} checks com falha não listados.",
    },
    create:
      "A branch {branch} ainda não tem pull request. Abra um com `gh pr create`: faça o push da branch antes se ela não estiver no remoto, escreva o título e o corpo a partir dos commits e do diff contra a branch base, e siga o template de pull request do repositório, se houver.",
  },
  delegate: {
    session: 'Delegue para a sessão "{target}": ',
    newWorktree: "Delegue para uma nova sessão em worktree: ",
    newWorktreeWithCommand:
      "Delegue para uma nova sessão em worktree (rode `lich open --worktree <branch> --prompt <task>`): ",
  },
  reviewComments: {
    header: "Comentários de revisão:",
  },
  issue: {
    head: "Issue do GitHub #{number}: {title}\n{url}",
  },
} satisfies Shape<typeof en>
